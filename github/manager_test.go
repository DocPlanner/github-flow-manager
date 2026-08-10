package github

import (
	"testing"
	"time"

	"github.com/shurcooL/githubv4"
)

// These began as characterization tests pinning the behaviour of hydrateCommits and
// PickFirstParentCommits before the fix for issue #36, defects included. They were then ported to
// the reshaped query with exactly the assertions for those defects flipped — each marked
// "WAS A DEFECT, NOW FIXED" and stating what it used to assert. Every other expectation is
// unchanged from the pre-fix run, which is what makes them useful: a different one moving means
// something regressed.
//
// Not covered here, deliberately: whether the query fetches enough of a commit's checks in the
// first place. That is a property of the GraphQL query, not of hydrateCommits — which only ever
// sees whatever was fetched — so no fixture can express it. TestChecksResolvedAcrossPages covers
// the resolution half, and testdata/baseline/ covers it end to end against real repositories.

// success is the only value hydrateCommits/checkRunSet treat as a passing check.
const success = githubv4.String(githubv4.StatusStateSuccess)

// --- fixture builders -------------------------------------------------------

func newQuery(nodes ...EdgeRootNode) *Query {
	q := &Query{}
	for _, n := range nodes {
		q.Repository.Ref.Target.Commit.History.Edges = append(
			q.Repository.Ref.Target.Commit.History.Edges,
			Edge{Node: n},
		)
	}
	return q
}

func commitNode(sha, message string) EdgeRootNode {
	return EdgeRootNode{
		Oid:     githubv4.String(sha),
		Message: githubv4.String(message),
	}
}

// withStatusContext adds a classic commit status to Status.Contexts.
func withStatusContext(n EdgeRootNode, name, state string) EdgeRootNode {
	n.Status.Contexts = append(n.Status.Contexts, Context{
		Context: githubv4.String(name),
		State:   githubv4.String(state),
	})
	return n
}

// checkRun builds a completed CheckRun member of the statusCheckRollup contexts union.
func checkRun(name, conclusion string) RollupContext {
	return RollupContext{
		Typename: "CheckRun",
		CheckRun: RollupCheckRun{
			Name:       githubv4.String(name),
			Status:     githubv4.String(githubv4.CheckStatusStateCompleted),
			Conclusion: githubv4.String(conclusion),
		},
	}
}

// pendingCheckRun builds a CheckRun that has not finished, so it has no conclusion yet.
func pendingCheckRun(name string) RollupContext {
	return RollupContext{
		Typename: "CheckRun",
		CheckRun: RollupCheckRun{
			Name:   githubv4.String(name),
			Status: githubv4.String(githubv4.CheckStatusStateInProgress),
		},
	}
}

// rollupStatusContext builds the StatusContext member of the same union — a classic commit status
// as reported through the rollup rather than through Commit.status.
func rollupStatusContext(name, state string) RollupContext {
	return RollupContext{
		Typename: "StatusContext",
		StatusContext: RollupStatusContext{
			Context: githubv4.String(name),
			State:   githubv4.String(state),
		},
	}
}

// withCheckRuns adds entries to the commit's statusCheckRollup contexts, which is where check runs
// are read from.
func withCheckRuns(n EdgeRootNode, runs ...RollupContext) EdgeRootNode {
	n.StatusCheckRollup.Contexts.Nodes = append(n.StatusCheckRollup.Contexts.Nodes, runs...)
	return n
}

// withWorkflowSuite adds a check suite identified by its workflow name, whose
// verdict lives in WorkflowRun.CheckSuite.Conclusion.
func withWorkflowSuite(n EdgeRootNode, workflowName, conclusion string) EdgeRootNode {
	n.CheckSuites.Nodes = append(n.CheckSuites.Nodes, CheckSuiteNode{
		WorkflowRun: WorkflowRun{
			Workflow:   Workflow{Name: githubv4.String(workflowName)},
			CheckSuite: CheckSuite{Conclusion: githubv4.String(conclusion)},
		},
	})
	return n
}

func withRollup(n EdgeRootNode, state string) EdgeRootNode {
	n.StatusCheckRollup.State = githubv4.String(state)
	return n
}

func withParents(n EdgeRootNode, shaMessagePairs ...string) EdgeRootNode {
	for i := 0; i < len(shaMessagePairs); i += 2 {
		n.Parents.Edges = append(n.Parents.Edges, EdgeParent{Node: EdgeNode{
			Oid:     githubv4.String(shaMessagePairs[i]),
			Message: githubv4.String(shaMessagePairs[i+1]),
		}})
	}
	return n
}

// --- hydrateCommits ---------------------------------------------------------

func TestHydrateCommits(t *testing.T) {
	tests := []struct {
		name   string
		nodes  []EdgeRootNode
		checks string
		sep    string
		want   []bool
	}{
		{
			name:   "classic commit status with SUCCESS state passes",
			nodes:  []EdgeRootNode{withStatusContext(commitNode("sha1", "msg1"), "ci", string(success))},
			checks: "ci",
			sep:    ",",
			want:   []bool{true},
		},
		{
			name:   "check run with SUCCESS conclusion passes",
			nodes:  []EdgeRootNode{withCheckRuns(commitNode("sha1", "msg1"), checkRun("ci", string(success)))},
			checks: "ci",
			sep:    ",",
			want:   []bool{true},
		},
		{
			name:   "workflow name match with SUCCESS check suite conclusion passes",
			nodes:  []EdgeRootNode{withWorkflowSuite(commitNode("sha1", "msg1"), "ci", string(success))},
			checks: "ci",
			sep:    ",",
			want:   []bool{true},
		},
		{
			// WAS A DEFECT, NOW FIXED: this expectation was false. The name was found twice —
			// once as a classic status, once as a check run — which drove checksPassed to 2
			// against numChecks of 1, so the old `checksPassed == numChecks` gate rejected a
			// commit whose check had in fact succeeded. State per name replaces the counter.
			// Observed live on a consumer repository that publishes its aggregate required check
			// both ways on the same commit.
			name: "same name as both classic status and check run passes",
			nodes: []EdgeRootNode{withCheckRuns(
				withStatusContext(commitNode("sha1", "msg1"), "ci", string(success)),
				checkRun("ci", string(success)),
			)},
			checks: "ci",
			sep:    ",",
			want:   []bool{true},
		},
		{
			// WAS A DEFECT, NOW FIXED: this expectation was false. Re-runs produce several check
			// runs of the same name and each success used to increment the counter past numChecks.
			name: "duplicate successful check runs of the same name pass",
			nodes: []EdgeRootNode{withCheckRuns(
				commitNode("sha1", "msg1"),
				checkRun("ci", string(success)),
				checkRun("ci", string(success)),
			)},
			checks: "ci",
			sep:    ",",
			want:   []bool{true},
		},
		{
			// WAS A DEFECT, NOW FIXED: this expectation was false. As above, with the duplicates
			// arriving from separate sources rather than together.
			name: "duplicate successful check runs from separate sources pass",
			nodes: []EdgeRootNode{withCheckRuns(
				withCheckRuns(commitNode("sha1", "msg1"), checkRun("ci", string(success))),
				checkRun("ci", string(success)),
			)},
			checks: "ci",
			sep:    ",",
			want:   []bool{true},
		},
		{
			// A success anywhere satisfies the name: the old code already counted any matching
			// success, so a name that both failed and succeeded must keep passing.
			name: "name that failed in one place and succeeded in another passes",
			nodes: []EdgeRootNode{withCheckRuns(
				commitNode("sha1", "msg1"),
				checkRun("ci", "FAILURE"),
				checkRun("ci", string(success)),
			)},
			checks: "ci",
			sep:    ",",
			want:   []bool{true},
		},
		{
			name: "classic status reported through the rollup passes",
			nodes: []EdgeRootNode{withCheckRuns(
				commitNode("sha1", "msg1"),
				rollupStatusContext("ci", string(success)),
			)},
			checks: "ci",
			sep:    ",",
			want:   []bool{true},
		},
		{
			name: "check run still running does not pass",
			nodes: []EdgeRootNode{withCheckRuns(
				commitNode("sha1", "msg1"),
				pendingCheckRun("ci"),
			)},
			checks: "ci",
			sep:    ",",
			want:   []bool{false},
		},
		{
			// A missing check is indistinguishable from a failing one.
			name: "requested name absent from every source fails",
			nodes: []EdgeRootNode{withCheckRuns(
				withStatusContext(commitNode("sha1", "msg1"), "other-status", string(success)),
				checkRun("other-run", string(success)),
			)},
			checks: "ci",
			sep:    ",",
			want:   []bool{false},
		},
		{
			name:   "check run with FAILURE conclusion fails",
			nodes:  []EdgeRootNode{withCheckRuns(commitNode("sha1", "msg1"), checkRun("ci", "FAILURE"))},
			checks: "ci",
			sep:    ",",
			want:   []bool{false},
		},
		{
			name:   "check run with NEUTRAL conclusion fails",
			nodes:  []EdgeRootNode{withCheckRuns(commitNode("sha1", "msg1"), checkRun("ci", "NEUTRAL"))},
			checks: "ci",
			sep:    ",",
			want:   []bool{false},
		},
		{
			name:   "check run with SKIPPED conclusion fails",
			nodes:  []EdgeRootNode{withCheckRuns(commitNode("sha1", "msg1"), checkRun("ci", "SKIPPED"))},
			checks: "ci",
			sep:    ",",
			want:   []bool{false},
		},
		{
			name:   "classic commit status with FAILURE state fails",
			nodes:  []EdgeRootNode{withStatusContext(commitNode("sha1", "msg1"), "ci", "FAILURE")},
			checks: "ci",
			sep:    ",",
			want:   []bool{false},
		},
		{
			// With no requested names the rollup state decides. Note the counting
			// block still runs first (strings.Split("", ",") yields [""], so
			// numChecks is 1) but its verdict is unconditionally overwritten here.
			name:   "empty checks names uses SUCCESS rollup state",
			nodes:  []EdgeRootNode{withRollup(commitNode("sha1", "msg1"), string(success))},
			checks: "",
			sep:    ",",
			want:   []bool{true},
		},
		{
			name:   "empty checks names uses FAILURE rollup state",
			nodes:  []EdgeRootNode{withRollup(commitNode("sha1", "msg1"), "FAILURE")},
			checks: "",
			sep:    ",",
			want:   []bool{false},
		},
		{
			// The rollup branch overwrites the counting result, so even a commit
			// whose checks would have counted as passing follows the rollup.
			name: "empty checks names ignores passing checks and follows the rollup",
			nodes: []EdgeRootNode{withRollup(
				withStatusContext(commitNode("sha1", "msg1"), "", string(success)),
				"FAILURE",
			)},
			checks: "",
			sep:    ",",
			want:   []bool{false},
		},
		{
			name: "two requested names with only one passing fails",
			nodes: []EdgeRootNode{withCheckRuns(
				commitNode("sha1", "msg1"),
				checkRun("a", string(success)),
				checkRun("b", "FAILURE"),
			)},
			checks: "a,b",
			sep:    ",",
			want:   []bool{false},
		},
		{
			name: "two requested names each passing once via a different source succeeds",
			nodes: []EdgeRootNode{withCheckRuns(
				withStatusContext(commitNode("sha1", "msg1"), "a", string(success)),
				checkRun("b", string(success)),
			)},
			checks: "a,b",
			sep:    ",",
			want:   []bool{true},
		},
		{
			name: "custom separator splits the requested names",
			nodes: []EdgeRootNode{withCheckRuns(
				withStatusContext(commitNode("sha1", "msg1"), "a", string(success)),
				checkRun("b", string(success)),
			)},
			checks: "a|b",
			sep:    "|",
			want:   []bool{true},
		},
		{
			name: "each commit in the history is evaluated independently and in order",
			nodes: []EdgeRootNode{
				withCheckRuns(commitNode("sha1", "msg1"), checkRun("ci", string(success))),
				withCheckRuns(commitNode("sha2", "msg2"), checkRun("ci", "FAILURE")),
				withStatusContext(commitNode("sha3", "msg3"), "ci", string(success)),
			},
			checks: "ci",
			sep:    ",",
			want:   []bool{true, false, true},
		},
		{
			name:   "empty history returns no commits",
			nodes:  nil,
			checks: "ci",
			sep:    ",",
			want:   nil,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got := hydrateCommits(newQuery(tt.nodes...), tt.checks, tt.sep)

			if len(got) != len(tt.want) {
				t.Fatalf("got %d commits, want %d", len(got), len(tt.want))
			}
			for i, want := range tt.want {
				if got[i].StatusSuccess != want {
					t.Errorf("commit %d (%s): StatusSuccess = %t, want %t",
						i, got[i].SHA, got[i].StatusSuccess, want)
				}
			}
		})
	}

	t.Run("maps commit fields", func(t *testing.T) {
		authored := time.Date(2021, time.June, 1, 12, 30, 0, 0, time.UTC)
		node := commitNode("abc123", "feat: something")
		node.AuthoredDate = githubv4.DateTime{Time: authored}
		node.Author = Author{Name: "Ada Lovelace"}

		got := hydrateCommits(newQuery(node), "ci", ",")
		if len(got) != 1 {
			t.Fatalf("got %d commits, want 1", len(got))
		}
		if got[0].SHA != "abc123" {
			t.Errorf("SHA = %q, want %q", got[0].SHA, "abc123")
		}
		if got[0].Message != "feat: something" {
			t.Errorf("Message = %q, want %q", got[0].Message, "feat: something")
		}
		if got[0].AuthorName != "Ada Lovelace" {
			t.Errorf("AuthorName = %q, want %q", got[0].AuthorName, "Ada Lovelace")
		}
		if !got[0].AuthoredDate.Equal(authored) {
			t.Errorf("AuthoredDate = %v, want %v", got[0].AuthoredDate, authored)
		}
	})

	t.Run("reports why each requested check did not pass", func(t *testing.T) {
		node := withWorkflowSuite(
			withCheckRuns(
				commitNode("sha1", "msg1"),
				checkRun("green", string(success)),
				checkRun("red", "FAILURE"),
				pendingCheckRun("running"),
			),
			"workflow-green", string(success),
		)

		got := hydrateCommits(newQuery(node), "green,red,running,absent,workflow-green", ",")
		if len(got) != 1 {
			t.Fatalf("got %d commits, want 1", len(got))
		}

		want := []CheckResult{
			{Name: "green", State: CheckPassed},
			{Name: "red", State: CheckFailed},
			{Name: "running", State: CheckPending},
			{Name: "absent", State: CheckNotFound},
			{Name: "workflow-green", State: CheckPassed},
		}
		if len(got[0].CheckResults) != len(want) {
			t.Fatalf("got %d results %v, want %d", len(got[0].CheckResults), got[0].CheckResults, len(want))
		}
		for i, w := range want {
			if got[0].CheckResults[i] != w {
				t.Errorf("result %d = %+v, want %+v", i, got[0].CheckResults[i], w)
			}
		}
		if got[0].StatusSuccess {
			t.Errorf("StatusSuccess = true, want false")
		}
	})

	t.Run("maps parents", func(t *testing.T) {
		node := withParents(commitNode("head", "head msg"), "p1", "parent one", "p2", "parent two")

		got := hydrateCommits(newQuery(node), "ci", ",")
		if len(got) != 1 {
			t.Fatalf("got %d commits, want 1", len(got))
		}
		parents := got[0].Parents
		if len(parents) != 2 {
			t.Fatalf("got %d parents, want 2", len(parents))
		}
		if parents[0].SHA != "p1" || parents[0].Message != "parent one" {
			t.Errorf("parent 0 = {%q, %q}, want {%q, %q}",
				parents[0].SHA, parents[0].Message, "p1", "parent one")
		}
		if parents[1].SHA != "p2" || parents[1].Message != "parent two" {
			t.Errorf("parent 1 = {%q, %q}, want {%q, %q}",
				parents[1].SHA, parents[1].Message, "p2", "parent two")
		}
	})

	t.Run("commit with no parents has nil parents", func(t *testing.T) {
		got := hydrateCommits(newQuery(commitNode("head", "head msg")), "ci", ",")
		if len(got) != 1 {
			t.Fatalf("got %d commits, want 1", len(got))
		}
		if got[0].Parents != nil {
			t.Errorf("Parents = %v, want nil", got[0].Parents)
		}
	})
}

// --- paging over a commit's check data --------------------------------------

// A commit whose check data spans several pages must be judged on all of it. This covers the
// resolution half of that contract — the part TopUpChecks performs once it has fetched a page —
// without going near the network: state accumulates across pages and settle() re-publishes it.
func TestChecksResolvedAcrossPages(t *testing.T) {
	firstPage := withCheckRuns(commitNode("sha1", "msg1"), checkRun("a", string(success)))
	firstPage.StatusCheckRollup.Contexts.PageInfo = PageInfo{HasNextPage: true, EndCursor: "cursor1"}

	commits := hydrateCommits(newQuery(firstPage), "a,b", ",")
	if len(commits) != 1 {
		t.Fatalf("got %d commits, want 1", len(commits))
	}
	c := &commits[0]

	if c.StatusSuccess {
		t.Errorf("StatusSuccess = true after first page, want false: b has not been seen yet")
	}
	if !c.ChecksTruncated() {
		t.Fatalf("ChecksTruncated() = false, want true: the rollup reported another page")
	}

	// What TopUpChecks does with the next page it fetches.
	applyRollupContexts(c.states, []RollupContext{checkRun("b", string(success))})
	c.settle()

	if !c.StatusSuccess {
		t.Errorf("StatusSuccess = false after the second page, want true: %+v", c.CheckResults)
	}
}

func TestChecksNotTruncatedWhenConnectionsAreComplete(t *testing.T) {
	commits := hydrateCommits(newQuery(withCheckRuns(commitNode("sha1", "msg1"), checkRun("a", string(success)))), "a", ",")
	if len(commits) != 1 {
		t.Fatalf("got %d commits, want 1", len(commits))
	}
	if commits[0].ChecksTruncated() {
		t.Errorf("ChecksTruncated() = true, want false: neither connection reported another page")
	}
}

// --- PickFirstParentCommits -------------------------------------------------

func TestPickFirstParentCommits(t *testing.T) {
	tests := []struct {
		name  string
		input []Commit
		want  []string
	}{
		{
			name:  "empty input returns nothing",
			input: nil,
			want:  nil,
		},
		{
			name:  "single commit without parents returns just that commit",
			input: []Commit{{SHA: "head"}},
			want:  []string{"head"},
		},
		{
			name: "linear chain is walked from HEAD down",
			input: []Commit{
				{SHA: "head", Parents: []Commit{{SHA: "a"}}},
				{SHA: "a", Parents: []Commit{{SHA: "b"}}},
				{SHA: "b"},
			},
			want: []string{"head", "a", "b"},
		},
		{
			name: "walk stops when a parent is missing from the list",
			input: []Commit{
				{SHA: "head", Parents: []Commit{{SHA: "a"}}},
				{SHA: "a", Parents: []Commit{{SHA: "beyond-the-page"}}},
			},
			want: []string{"head", "a"},
		},
		{
			name: "only the first parent of a merge commit is followed",
			input: []Commit{
				{SHA: "merge", Parents: []Commit{{SHA: "a"}, {SHA: "b"}}},
				{SHA: "a"},
				{SHA: "b"},
			},
			want: []string{"merge", "a"},
		},
		{
			name: "commits unreachable through first parents are dropped",
			input: []Commit{
				{SHA: "head", Parents: []Commit{{SHA: "a"}}},
				{SHA: "side", Parents: []Commit{{SHA: "a"}}},
				{SHA: "a"},
			},
			want: []string{"head", "a"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got := PickFirstParentCommits(tt.input)

			if len(got) != len(tt.want) {
				t.Fatalf("got %d commits %v, want %d %v", len(got), shas(got), len(tt.want), tt.want)
			}
			for i, want := range tt.want {
				if got[i].SHA != want {
					t.Errorf("commit %d: SHA = %q, want %q", i, got[i].SHA, want)
				}
			}
		})
	}
}

func shas(commits []Commit) []string {
	out := make([]string, 0, len(commits))
	for _, c := range commits {
		out = append(out, c.SHA)
	}
	return out
}
