package github

import (
	"strings"
	"testing"

	"github.com/shurcooL/githubv4"
)

// The required check names DocPlanner/dbt-app promotes on. They are used
// throughout these tests because the defect this file guards against was found
// on that repository.
const (
	checkBuildPush = "build_push / build_push-dbt"
	checkAppTests  = "run_app_tests"
	checkDbtTests  = "run_dbt_tests"
)

var dbtAppChecks = []string{checkBuildPush, checkAppTests, checkDbtTests}

// completedRun builds a check run that finished with the given conclusion.
func completedRun(name string, conclusion githubv4.CheckConclusionState) RollupContext {
	return RollupContext{
		Typename: "CheckRun",
		CheckRun: RollupCheckRun{
			Name:       githubv4.String(name),
			Status:     githubv4.String(githubv4.CheckStatusStateCompleted),
			Conclusion: githubv4.String(conclusion),
		},
	}
}

// unfinishedRun builds a check run that has not concluded yet, the way GitHub
// reports it: a non-completed status and a null conclusion.
func unfinishedRun(name string, status githubv4.CheckStatusState) RollupContext {
	return RollupContext{
		Typename: "CheckRun",
		CheckRun: RollupCheckRun{
			Name:   githubv4.String(name),
			Status: githubv4.String(status),
		},
	}
}

// edgeWithRuns puts check runs where GitHub reports them for our purposes: the status-check
// rollup, a single flat list per commit regardless of which suite each run came from.
func edgeWithRuns(runs ...RollupContext) Edge {
	return Edge{Node: EdgeRootNode{StatusCheckRollup: StatusCheckRollup{
		Contexts: StatusCheckRollupContexts{Nodes: runs},
	}}}
}

// edgeWithStatusContexts builds a commit carrying classic commit statuses rather
// than check runs.
func edgeWithStatusContexts(contexts ...Context) Edge {
	return Edge{Node: EdgeRootNode{Status: NodeStatus{Contexts: contexts}}}
}

// edgeWithWorkflowRun builds a commit whose required name is the name of a
// workflow, so the verdict lives on the check suite of its workflow run.
func edgeWithWorkflowRun(workflow string, conclusion githubv4.CheckConclusionState) Edge {
	return Edge{Node: EdgeRootNode{CheckSuites: CheckSuites{Nodes: []CheckSuiteNode{{
		WorkflowRun: WorkflowRun{
			Workflow:   Workflow{Name: githubv4.String(workflow)},
			CheckSuite: CheckSuite{Conclusion: githubv4.String(conclusion)},
		},
	}}}}}
}

// TestDuplicateSuccessNoLongerMasksAFailure covers the historical false pass.
//
// The old implementation summed successes across all names and compared the
// total to the number of names, so two successful runs of one check paid for a
// second check that had failed outright: 1 + 2 + 0 = 3 == 3 names, promoted.
func TestDuplicateSuccessNoLongerMasksAFailure(t *testing.T) {
	edge := edgeWithRuns(
		completedRun(checkBuildPush, githubv4.CheckConclusionStateSuccess),
		completedRun(checkAppTests, githubv4.CheckConclusionStateSuccess),
		completedRun(checkAppTests, githubv4.CheckConclusionStateSuccess),
		completedRun(checkDbtTests, githubv4.CheckConclusionStateFailure),
	)

	statusSuccess, summary := evaluateSpecificChecks(sourcesFromEdge(edge), dbtAppChecks, false)

	if statusSuccess {
		t.Fatalf("a commit whose %s failed must not be promotable, summary: %v", checkDbtTests, summary)
	}
	assertSummaryMentions(t, summary, checkDbtTests, "did not pass")
}

// TestDuplicateSuccessNoLongerMasksAMissingCheck covers the same false pass for
// a required check that never produced any run at all, which is what the
// evidence commit 7d7d1c8b looked like from the gate's point of view: the total
// reached the required count while one name had never gone green.
func TestDuplicateSuccessNoLongerMasksAMissingCheck(t *testing.T) {
	edge := edgeWithRuns(
		completedRun(checkBuildPush, githubv4.CheckConclusionStateSuccess),
		completedRun(checkAppTests, githubv4.CheckConclusionStateSuccess),
		completedRun(checkAppTests, githubv4.CheckConclusionStateSuccess),
	)

	statusSuccess, summary := evaluateSpecificChecks(sourcesFromEdge(edge), dbtAppChecks, false)

	if statusSuccess {
		t.Fatalf("a commit missing %s entirely must not be promotable, summary: %v", checkDbtTests, summary)
	}
	assertSummaryMentions(t, summary, checkDbtTests, "NEVER RAN")
}

// TestExtraSuccessfulRunNoLongerBlocks covers the historical false block. Every
// required check passed, but one of them ran twice, so the old exact-equality
// test saw 4 successes against 3 names and refused to promote.
func TestExtraSuccessfulRunNoLongerBlocks(t *testing.T) {
	edge := edgeWithRuns(
		completedRun(checkBuildPush, githubv4.CheckConclusionStateSuccess),
		completedRun(checkAppTests, githubv4.CheckConclusionStateSuccess),
		completedRun(checkAppTests, githubv4.CheckConclusionStateSuccess),
		completedRun(checkDbtTests, githubv4.CheckConclusionStateSuccess),
	)

	statusSuccess, summary := evaluateSpecificChecks(sourcesFromEdge(edge), dbtAppChecks, false)

	if !statusSuccess {
		t.Fatalf("every required check passed, so a duplicate run must not block promotion, summary: %v", summary)
	}
}

// TestSkippedPolicyIsOptIn pins down the one deliberate policy choice here.
//
// The fixture is the exact evidence from DocPlanner/dbt-app commit 7d7d1c8b,
// built while a merge queue was active: run_app_tests reported twice (queue run
// plus push run) and run_dbt_tests skipped. The old rule counted 1 + 2 + 0 = 3
// against 3 names and promoted it, even though run_dbt_tests had never gone
// green.
//
// SKIPPED does not satisfy a required check by default, which is what the old
// rule did too - it only ever counted SUCCESS. Repositories that skip a required
// job on purpose opt in with --accept-skipped-checks.
func TestSkippedPolicyIsOptIn(t *testing.T) {
	edge := edgeWithRuns(
		completedRun(checkBuildPush, githubv4.CheckConclusionStateSuccess),
		completedRun(checkAppTests, githubv4.CheckConclusionStateSuccess),
		completedRun(checkAppTests, githubv4.CheckConclusionStateSuccess),
		completedRun(checkDbtTests, githubv4.CheckConclusionStateSkipped),
	)

	statusSuccess, summary := evaluateSpecificChecks(sourcesFromEdge(edge), dbtAppChecks, false)
	if statusSuccess {
		t.Fatalf("by default a skipped required check must not satisfy the gate, summary: %v", summary)
	}
	assertSummaryMentions(t, summary, checkDbtTests, "did not pass")

	if statusSuccess, summary := evaluateSpecificChecks(sourcesFromEdge(edge), dbtAppChecks, true); !statusSuccess {
		t.Fatalf("with acceptSkippedChecks a skipped required check must be accepted, summary: %v", summary)
	}
}

// TestNeutralNeverSatisfies guards a fail-closed gate that several repositories
// depend on and that no flag may open.
//
// devops-pipelines' hotfix_aware_skip_check workflow publishes its
// hotfix-skip-tests check as SUCCESS to mean "this commit is authorised to
// promote without tests" and NEUTRAL to mean "not a hotfix". monolith-app gates
// its hotfix promote on that single name with StatusSuccess == true, so treating
// NEUTRAL as satisfied would let any ordinary commit promote untested.
func TestNeutralNeverSatisfies(t *testing.T) {
	const hotfixSkipTests = "hotfix-skip-tests"
	names := []string{hotfixSkipTests}

	for _, acceptSkipped := range []bool{false, true} {
		edge := edgeWithRuns(completedRun(hotfixSkipTests, githubv4.CheckConclusionStateNeutral))

		statusSuccess, summary := evaluateSpecificChecks(sourcesFromEdge(edge), names, acceptSkipped)
		if statusSuccess {
			t.Fatalf("a neutral %s must never authorise promotion (acceptSkippedChecks=%t), summary: %v", hotfixSkipTests, acceptSkipped, summary)
		}
	}

	// And the authorised case still promotes.
	edge := edgeWithRuns(completedRun(hotfixSkipTests, githubv4.CheckConclusionStateSuccess))
	if statusSuccess, summary := evaluateSpecificChecks(sourcesFromEdge(edge), names, false); !statusSuccess {
		t.Fatalf("a successful %s must authorise promotion, summary: %v", hotfixSkipTests, summary)
	}
}

// TestSingleNameReportedTwiceStillPromotes covers the false block for the very
// common single-required-name repositories. A reusable workflow called by several
// callers publishes its check once per caller, so one name routinely resolves to
// several successful results - and the old exact-equality test then saw 2 != 1
// and refused to promote a fully green commit. monolith-app's hotfix path
// (hotfix-skip-tests) is one of these.
func TestSingleNameReportedTwiceStillPromotes(t *testing.T) {
	const loadContext = "load_context / Load context"
	edge := edgeWithRuns(
		completedRun(loadContext, githubv4.CheckConclusionStateSuccess),
		completedRun(loadContext, githubv4.CheckConclusionStateSuccess),
		completedRun(loadContext, githubv4.CheckConclusionStateSuccess),
		completedRun(loadContext, githubv4.CheckConclusionStateSuccess),
	)

	statusSuccess, summary := evaluateSpecificChecks(sourcesFromEdge(edge), []string{loadContext}, false)
	if !statusSuccess {
		t.Fatalf("four successful results for the only required name must promote, summary: %v", summary)
	}
}

// TestPendingResultsBlock makes sure the gate never pre-empts a verdict, whether
// GitHub signals "not finished" through the run's status or through a null
// conclusion.
func TestPendingResultsBlock(t *testing.T) {
	tests := map[string]Edge{
		"queued check run": edgeWithRuns(
			completedRun(checkBuildPush, githubv4.CheckConclusionStateSuccess),
			completedRun(checkAppTests, githubv4.CheckConclusionStateSuccess),
			unfinishedRun(checkDbtTests, githubv4.CheckStatusStateQueued),
		),
		"in progress check run": edgeWithRuns(
			completedRun(checkBuildPush, githubv4.CheckConclusionStateSuccess),
			completedRun(checkAppTests, githubv4.CheckConclusionStateSuccess),
			unfinishedRun(checkDbtTests, githubv4.CheckStatusStateInProgress),
		),
		"successful run alongside one still in progress": edgeWithRuns(
			completedRun(checkBuildPush, githubv4.CheckConclusionStateSuccess),
			completedRun(checkAppTests, githubv4.CheckConclusionStateSuccess),
			completedRun(checkDbtTests, githubv4.CheckConclusionStateSuccess),
			unfinishedRun(checkDbtTests, githubv4.CheckStatusStateInProgress),
		),
		"workflow run with no conclusion yet": Edge{Node: EdgeRootNode{
			StatusCheckRollup: StatusCheckRollup{Contexts: StatusCheckRollupContexts{Nodes: []RollupContext{
				completedRun(checkBuildPush, githubv4.CheckConclusionStateSuccess),
				completedRun(checkAppTests, githubv4.CheckConclusionStateSuccess),
			}}},
			CheckSuites: CheckSuites{Nodes: []CheckSuiteNode{
				{WorkflowRun: WorkflowRun{Workflow: Workflow{Name: githubv4.String(checkDbtTests)}}},
			}},
		}},
		"pending commit status": edgeWithStatusContexts(
			Context{Context: checkBuildPush, State: githubv4.String(githubv4.StatusStateSuccess)},
			Context{Context: checkAppTests, State: githubv4.String(githubv4.StatusStateSuccess)},
			Context{Context: checkDbtTests, State: githubv4.String(githubv4.StatusStatePending)},
		),
	}

	for name, edge := range tests {
		t.Run(name, func(t *testing.T) {
			statusSuccess, summary := evaluateSpecificChecks(sourcesFromEdge(edge), dbtAppChecks, false)
			if statusSuccess {
				t.Fatalf("promoting now would pre-empt the verdict of %s, summary: %v", checkDbtTests, summary)
			}
		})
	}
}

// TestFailingConclusionsBlock walks every conclusion that must stop a promotion,
// each reported for one required name while the other two are green.
func TestFailingConclusionsBlock(t *testing.T) {
	blocking := []githubv4.CheckConclusionState{
		githubv4.CheckConclusionStateFailure,
		githubv4.CheckConclusionStateCancelled,
		githubv4.CheckConclusionStateTimedOut,
		githubv4.CheckConclusionStateActionRequired,
		githubv4.CheckConclusionStateStale,
		githubv4.CheckConclusionStateStartupFailure,
		githubv4.CheckConclusionStateNeutral,
		githubv4.CheckConclusionStateSkipped,
	}

	for _, conclusion := range blocking {
		t.Run(string(conclusion), func(t *testing.T) {
			edge := edgeWithRuns(
				completedRun(checkBuildPush, githubv4.CheckConclusionStateSuccess),
				completedRun(checkAppTests, githubv4.CheckConclusionStateSuccess),
				completedRun(checkDbtTests, conclusion),
			)

			statusSuccess, summary := evaluateSpecificChecks(sourcesFromEdge(edge), dbtAppChecks, false)
			if statusSuccess {
				t.Fatalf("%s must not be treated as green, summary: %v", conclusion, summary)
			}
		})
	}
}

// TestAllGreenPasses is the happy path, once per source GitHub can report a
// required name through.
func TestAllGreenPasses(t *testing.T) {
	tests := map[string]Edge{
		"check runs": edgeWithRuns(
			completedRun(checkBuildPush, githubv4.CheckConclusionStateSuccess),
			completedRun(checkAppTests, githubv4.CheckConclusionStateSuccess),
			completedRun(checkDbtTests, githubv4.CheckConclusionStateSuccess),
		),
		"commit statuses": edgeWithStatusContexts(
			Context{Context: checkBuildPush, State: githubv4.String(githubv4.StatusStateSuccess)},
			Context{Context: checkAppTests, State: githubv4.String(githubv4.StatusStateSuccess)},
			Context{Context: checkDbtTests, State: githubv4.String(githubv4.StatusStateSuccess)},
		),
	}

	for name, edge := range tests {
		t.Run(name, func(t *testing.T) {
			statusSuccess, summary := evaluateSpecificChecks(sourcesFromEdge(edge), dbtAppChecks, false)
			if !statusSuccess {
				t.Fatalf("all required checks are green, summary: %v", summary)
			}
			for _, line := range summary {
				if !strings.HasPrefix(line, "OK ") {
					t.Errorf("expected every summary line to report OK, got %q", line)
				}
			}
		})
	}
}

// TestErroredCommitStatusBlocks covers the commit-status path for a red verdict,
// which has no conclusion of its own, only a state.
func TestErroredCommitStatusBlocks(t *testing.T) {
	for _, state := range []githubv4.StatusState{githubv4.StatusStateFailure, githubv4.StatusStateError} {
		t.Run(string(state), func(t *testing.T) {
			edge := edgeWithStatusContexts(
				Context{Context: checkBuildPush, State: githubv4.String(githubv4.StatusStateSuccess)},
				Context{Context: checkAppTests, State: githubv4.String(githubv4.StatusStateSuccess)},
				Context{Context: checkDbtTests, State: githubv4.String(state)},
			)

			statusSuccess, summary := evaluateSpecificChecks(sourcesFromEdge(edge), dbtAppChecks, false)
			if statusSuccess {
				t.Fatalf("a commit status of %s must block promotion, summary: %v", state, summary)
			}
		})
	}
}

// TestWorkflowRunNameIsMatched keeps the workflow-name path working: a required
// name can be the name of a whole workflow rather than of a single check run.
func TestWorkflowRunNameIsMatched(t *testing.T) {
	names := []string{checkAppTests}

	if statusSuccess, summary := evaluateSpecificChecks(sourcesFromEdge(edgeWithWorkflowRun(checkAppTests, githubv4.CheckConclusionStateSuccess)), names, false); !statusSuccess {
		t.Fatalf("a successful workflow run must satisfy its required name, summary: %v", summary)
	}

	statusSuccess, summary := evaluateSpecificChecks(sourcesFromEdge(edgeWithWorkflowRun(checkAppTests, githubv4.CheckConclusionStateFailure)), names, false)
	if statusSuccess {
		t.Fatalf("a failed workflow run must block promotion, summary: %v", summary)
	}
}

// TestMissingNameBlocks checks the simplest case of all: a required name that
// nothing on the commit reports.
func TestMissingNameBlocks(t *testing.T) {
	edge := edgeWithRuns(completedRun("some_other_check", githubv4.CheckConclusionStateSuccess))

	statusSuccess, summary := evaluateSpecificChecks(sourcesFromEdge(edge), []string{checkAppTests}, false)
	if statusSuccess {
		t.Fatalf("a required check that never ran must block promotion, summary: %v", summary)
	}
	assertSummaryMentions(t, summary, checkAppTests, "NEVER RAN")
}

// TestNoUsableCheckNamesFailsClosed makes sure an empty requirement set is never
// read as "everything passed".
func TestNoUsableCheckNamesFailsClosed(t *testing.T) {
	edge := edgeWithRuns(completedRun(checkAppTests, githubv4.CheckConclusionStateSuccess))

	statusSuccess, summary := evaluateSpecificChecks(sourcesFromEdge(edge), nil, false)
	if statusSuccess {
		t.Fatalf("an empty set of required names must not promote anything, summary: %v", summary)
	}
}

func TestSplitCheckNames(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		sep      string
		expected []string
	}{
		{name: "single name", input: checkAppTests, sep: ",", expected: []string{checkAppTests}},
		{
			name:     "the dbt-app list",
			input:    "run_app_tests,run_dbt_tests,build_push / build_push-dbt",
			sep:      ",",
			expected: []string{checkAppTests, checkDbtTests, checkBuildPush},
		},
		{name: "surrounding whitespace is dropped", input: "run_app_tests, run_dbt_tests", sep: ",", expected: []string{checkAppTests, checkDbtTests}},
		{name: "trailing separator adds no phantom name", input: "run_app_tests,run_dbt_tests,", sep: ",", expected: []string{checkAppTests, checkDbtTests}},
		{name: "custom separator", input: "run_app_tests;run_dbt_tests", sep: ";", expected: []string{checkAppTests, checkDbtTests}},
		{name: "nothing usable", input: " , ", sep: ",", expected: nil},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := splitCheckNames(test.input, test.sep)
			if len(got) != len(test.expected) {
				t.Fatalf("splitCheckNames(%q, %q) = %v, want %v", test.input, test.sep, got, test.expected)
			}
			for i, name := range got {
				if name != test.expected[i] {
					t.Errorf("splitCheckNames(%q, %q)[%d] = %q, want %q", test.input, test.sep, i, name, test.expected[i])
				}
			}
		})
	}
}

// TestHydrateCommitsFallsBackToRollup guards the untouched path: with no
// specific check names, StatusSuccess still comes straight from GitHub's status
// rollup and no per-name summary is produced.
func TestHydrateCommitsFallsBackToRollup(t *testing.T) {
	tests := map[string]struct {
		rollupState githubv4.StatusState
		expected    bool
	}{
		"successful rollup": {rollupState: githubv4.StatusStateSuccess, expected: true},
		"failing rollup":    {rollupState: githubv4.StatusStateFailure, expected: false},
		"pending rollup":    {rollupState: githubv4.StatusStatePending, expected: false},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// A red check run on the commit as well, to prove the rollup alone
			// decides on this path.
			edge := edgeWithRuns(completedRun(checkAppTests, githubv4.CheckConclusionStateFailure))
			edge.Node.Oid = "cafebabe"
			edge.Node.StatusCheckRollup.State = githubv4.String(test.rollupState)

			commits := hydrateCommits(queryWithEdges(edge), "", ",", false)

			if len(commits) != 1 {
				t.Fatalf("expected exactly one hydrated commit, got %d", len(commits))
			}
			if commits[0].StatusSuccess != test.expected {
				t.Errorf("StatusSuccess = %t for a %s rollup, want %t", commits[0].StatusSuccess, test.rollupState, test.expected)
			}
			if len(commits[0].ChecksSummary) != 0 {
				t.Errorf("expected no per-name summary on the rollup path, got %v", commits[0].ChecksSummary)
			}
		})
	}
}

// TestHydrateCommitsWithSpecificChecks checks the wiring from the command-line
// string through to a commit's StatusSuccess and summary.
func TestHydrateCommitsWithSpecificChecks(t *testing.T) {
	edge := edgeWithRuns(
		completedRun(checkBuildPush, githubv4.CheckConclusionStateSuccess),
		completedRun(checkAppTests, githubv4.CheckConclusionStateSuccess),
		completedRun(checkAppTests, githubv4.CheckConclusionStateSuccess),
		completedRun(checkDbtTests, githubv4.CheckConclusionStateSkipped),
	)
	edge.Node.Oid = "7d7d1c8b"
	// A green rollup, to prove it is ignored once specific names are requested.
	edge.Node.StatusCheckRollup.State = githubv4.String(githubv4.StatusStateSuccess)
	names := strings.Join(dbtAppChecks, ",")

	commits := hydrateCommits(queryWithEdges(edge), names, ",", false)
	if len(commits) != 1 || commits[0].StatusSuccess {
		t.Fatalf("expected the commit to be blocked by its skipped required check, got %+v", commits)
	}
	if len(commits[0].ChecksSummary) != len(dbtAppChecks) {
		t.Errorf("expected one summary line per required name, got %v", commits[0].ChecksSummary)
	}
	assertSummaryMentions(t, commits[0].ChecksSummary, checkDbtTests, "did not pass")

	commits = hydrateCommits(queryWithEdges(edge), names, ",", true)
	if len(commits) != 1 || !commits[0].StatusSuccess {
		t.Fatalf("expected the commit to be promotable with acceptSkippedChecks, got %+v", commits)
	}
}

// queryWithEdges wraps commit edges in the response shape the GraphQL query
// returns.
func queryWithEdges(edges ...Edge) *Query {
	return &Query{Repository: Repository{Ref: Ref{Target: Target{Commit: TargetCommit{History: History{Edges: edges}}}}}}
}

// assertSummaryMentions fails the test unless some summary line names the given
// check and contains the given reason.
func assertSummaryMentions(t *testing.T, summary []string, name, reason string) {
	t.Helper()

	for _, line := range summary {
		if strings.Contains(line, name) && strings.Contains(line, reason) {
			return
		}
	}

	t.Errorf("expected a summary line about %q containing %q, got %v", name, reason, summary)
}
