package github

import (
	"testing"

	"github.com/shurcooL/githubv4"
)

// GitHub caps checkSuites and statusCheckRollup.contexts at 100 entries per page, and repositories
// with a lot of CI exceed that, so a commit can look unsatisfied only because the deciding check sat
// on a page nobody asked for. These cover the two halves of the contract that prevents a verdict
// being reached on a partial view: noticing that a page is missing, and folding a fetched page in.

func edgeWithMorePages(edge Edge, contextsNext, suitesNext bool) Edge {
	edge.Node.StatusCheckRollup.Contexts.PageInfo = PageInfo{
		HasNextPage: githubv4.Boolean(contextsNext),
		EndCursor:   "contexts-cursor",
	}
	edge.Node.CheckSuites.PageInfo = PageInfo{
		HasNextPage: githubv4.Boolean(suitesNext),
		EndCursor:   "suites-cursor",
	}

	return edge
}

func TestChecksTruncatedTracksBothConnections(t *testing.T) {
	tests := map[string]struct {
		contextsNext, suitesNext bool
		expected                 bool
	}{
		"both connections complete": {false, false, false},
		"more rollup contexts":      {true, false, true},
		"more check suites":         {false, true, true},
		"more of both":              {true, true, true},
	}

	for name, test := range tests {
		test := test
		t.Run(name, func(t *testing.T) {
			edge := edgeWithMorePages(
				edgeWithRuns(completedRun(checkAppTests, githubv4.CheckConclusionStateSuccess)),
				test.contextsNext, test.suitesNext,
			)

			commits := hydrateCommits(queryWithEdges(edge), checkAppTests, ",", false)
			if len(commits) != 1 {
				t.Fatalf("expected exactly one hydrated commit, got %d", len(commits))
			}
			if got := commits[0].ChecksTruncated(); got != test.expected {
				t.Errorf("ChecksTruncated() = %t, want %t", got, test.expected)
			}
		})
	}
}

// A commit with no required names never needs more data, whatever the cursors say: the rollup-state
// path does not read these connections at all.
func TestChecksTruncatedIgnoredWithoutRequiredNames(t *testing.T) {
	edge := edgeWithMorePages(edgeWithRuns(), true, true)

	commits := hydrateCommits(queryWithEdges(edge), "", ",", false)
	if len(commits) != 1 {
		t.Fatalf("expected exactly one hydrated commit, got %d", len(commits))
	}
	if len(commits[0].checkNames) != 0 {
		t.Fatalf("expected no required names, got %v", commits[0].checkNames)
	}
}

// The half of TopUpChecks that does not touch the network: a required name found only on a later
// page must flip the commit once that page is folded in.
func TestLaterPageSatisfiesARequiredName(t *testing.T) {
	edge := edgeWithMorePages(
		edgeWithRuns(completedRun(checkAppTests, githubv4.CheckConclusionStateSuccess)),
		true, false,
	)

	commits := hydrateCommits(queryWithEdges(edge), checkAppTests+","+checkDbtTests, ",", false)
	if len(commits) != 1 {
		t.Fatalf("expected exactly one hydrated commit, got %d", len(commits))
	}
	commit := &commits[0]

	if commit.StatusSuccess {
		t.Errorf("StatusSuccess = true on the first page alone, want false: %q has not been seen yet", checkDbtTests)
	}
	if !commit.ChecksTruncated() {
		t.Fatalf("ChecksTruncated() = false, want true: the rollup reported a further page")
	}

	// What TopUpChecks does with the page it fetches.
	commit.sources.rollupContexts = append(commit.sources.rollupContexts,
		completedRun(checkDbtTests, githubv4.CheckConclusionStateSuccess))
	commit.reevaluate()

	if !commit.StatusSuccess {
		t.Errorf("StatusSuccess = false after the second page, want true: %v", commit.ChecksSummary)
	}
}

// A later page must be able to settle a name in the other direction too, so that paging cannot turn
// a red check into a promotion.
func TestLaterPageCanRevealAFailure(t *testing.T) {
	edge := edgeWithMorePages(edgeWithRuns(), true, false)

	commits := hydrateCommits(queryWithEdges(edge), checkDbtTests, ",", false)
	commit := &commits[0]

	commit.sources.rollupContexts = append(commit.sources.rollupContexts,
		completedRun(checkDbtTests, githubv4.CheckConclusionStateFailure))
	commit.reevaluate()

	if commit.StatusSuccess {
		t.Errorf("StatusSuccess = true, want false: the later page carried a failure")
	}
}

// A workflow-name requirement is only satisfiable from the check-suites connection, which is the
// reason that connection is still selected separately from the rollup.
func TestLaterSuitePageSatisfiesAWorkflowName(t *testing.T) {
	edge := edgeWithMorePages(edgeWithRuns(), false, true)

	commits := hydrateCommits(queryWithEdges(edge), checkAppTests, ",", false)
	commit := &commits[0]

	if commit.StatusSuccess {
		t.Fatalf("StatusSuccess = true before any suite was read, want false")
	}

	commit.sources.suites = append(commit.sources.suites, CheckSuiteNode{WorkflowRun: WorkflowRun{
		Workflow:   Workflow{Name: githubv4.String(checkAppTests)},
		CheckSuite: CheckSuite{Conclusion: githubv4.String(githubv4.CheckConclusionStateSuccess)},
	}})
	commit.reevaluate()

	if !commit.StatusSuccess {
		t.Errorf("StatusSuccess = false after the later suite page, want true: %v", commit.ChecksSummary)
	}
}

func TestGetCommitsRejectsOutOfRangePageSizes(t *testing.T) {
	gm := New("token")

	tests := map[string]struct {
		contexts, suites int
	}{
		"contexts too large": {101, 50},
		"contexts too small": {0, 50},
		"suites too large":   {50, 101},
		"suites too small":   {50, 0},
	}

	for name, test := range tests {
		test := test
		t.Run(name, func(t *testing.T) {
			if _, err := gm.GetCommits("o", "r", "b", 25, "", ",", false, test.contexts, test.suites); err == nil {
				t.Errorf("expected an error for contexts=%d suites=%d, got none", test.contexts, test.suites)
			}
		})
	}
}
