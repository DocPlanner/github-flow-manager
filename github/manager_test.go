package github

import (
	"testing"
	"time"

	"github.com/shurcooL/githubv4"
)

// These are characterization tests: they pin the CURRENT behaviour of
// hydrateCommits and PickFirstParentCommits, defects included. Cases that assert
// a known defect are marked with a KNOWN DEFECT comment.
//
// Not covered here, deliberately: the defect where a required check falls outside
// checkSuites(first: 20) / checkRuns(first: 25) and is therefore never seen. That is a
// property of the GraphQL query, not of hydrateCommits — which only ever sees whatever
// was already fetched, so a fixture holding 21 suites would simply have all 21 read.
// It is covered instead by the end-to-end verdict baseline in testdata/baseline/.

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

func checkRun(name, conclusion string) CheckRunNodes {
	return CheckRunNodes{
		Name:       githubv4.String(name),
		Conclusion: githubv4.String(conclusion),
	}
}

// withCheckRunSuite adds a check suite whose WorkflowRun is the zero value, so
// checkRunSet only looks at its CheckRuns.Nodes.
func withCheckRunSuite(n EdgeRootNode, runs ...CheckRunNodes) EdgeRootNode {
	n.CheckSuites.Nodes = append(n.CheckSuites.Nodes, CheckSuiteNode{
		CheckRuns: CheckRuns{Nodes: runs},
	})
	return n
}

// withWorkflowSuite adds a check suite identified by its workflow name, whose
// verdict lives in WorkflowRun.CheckSuite.Conclusion.
func withWorkflowSuite(n EdgeRootNode, workflowName, conclusion string, runs ...CheckRunNodes) EdgeRootNode {
	n.CheckSuites.Nodes = append(n.CheckSuites.Nodes, CheckSuiteNode{
		WorkflowRun: WorkflowRun{
			Workflow:   Workflow{Name: githubv4.String(workflowName)},
			CheckSuite: CheckSuite{Conclusion: githubv4.String(conclusion)},
		},
		CheckRuns: CheckRuns{Nodes: runs},
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
			nodes:  []EdgeRootNode{withCheckRunSuite(commitNode("sha1", "msg1"), checkRun("ci", string(success)))},
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
			// KNOWN DEFECT (double count): the requested name is found twice —
			// once as a classic status, once as a check run — so checksPassed
			// reaches 2 while numChecks is 1, and the `checksPassed == numChecks`
			// gate rejects a commit whose check actually succeeded. A later step
			// will intentionally flip this expectation to true.
			name: "same name as both classic status and check run is double counted and fails",
			nodes: []EdgeRootNode{withCheckRunSuite(
				withStatusContext(commitNode("sha1", "msg1"), "ci", string(success)),
				checkRun("ci", string(success)),
			)},
			checks: "ci",
			sep:    ",",
			want:   []bool{false},
		},
		{
			// KNOWN DEFECT (double count): re-runs produce several check runs with
			// the same name; each SUCCESS increments the counter. A later step will
			// intentionally flip this expectation to true.
			name: "duplicate successful check runs in one suite are double counted and fail",
			nodes: []EdgeRootNode{withCheckRunSuite(
				commitNode("sha1", "msg1"),
				checkRun("ci", string(success)),
				checkRun("ci", string(success)),
			)},
			checks: "ci",
			sep:    ",",
			want:   []bool{false},
		},
		{
			// KNOWN DEFECT (double count): same as above, across two suites.
			// A later step will intentionally flip this expectation to true.
			name: "duplicate successful check runs across suites are double counted and fail",
			nodes: []EdgeRootNode{withCheckRunSuite(
				withCheckRunSuite(commitNode("sha1", "msg1"), checkRun("ci", string(success))),
				checkRun("ci", string(success)),
			)},
			checks: "ci",
			sep:    ",",
			want:   []bool{false},
		},
		{
			// A missing check is indistinguishable from a failing one.
			name: "requested name absent from every source fails",
			nodes: []EdgeRootNode{withCheckRunSuite(
				withStatusContext(commitNode("sha1", "msg1"), "other-status", string(success)),
				checkRun("other-run", string(success)),
			)},
			checks: "ci",
			sep:    ",",
			want:   []bool{false},
		},
		{
			name:   "check run with FAILURE conclusion fails",
			nodes:  []EdgeRootNode{withCheckRunSuite(commitNode("sha1", "msg1"), checkRun("ci", "FAILURE"))},
			checks: "ci",
			sep:    ",",
			want:   []bool{false},
		},
		{
			name:   "check run with NEUTRAL conclusion fails",
			nodes:  []EdgeRootNode{withCheckRunSuite(commitNode("sha1", "msg1"), checkRun("ci", "NEUTRAL"))},
			checks: "ci",
			sep:    ",",
			want:   []bool{false},
		},
		{
			name:   "check run with SKIPPED conclusion fails",
			nodes:  []EdgeRootNode{withCheckRunSuite(commitNode("sha1", "msg1"), checkRun("ci", "SKIPPED"))},
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
			nodes: []EdgeRootNode{withCheckRunSuite(
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
			nodes: []EdgeRootNode{withCheckRunSuite(
				withStatusContext(commitNode("sha1", "msg1"), "a", string(success)),
				checkRun("b", string(success)),
			)},
			checks: "a,b",
			sep:    ",",
			want:   []bool{true},
		},
		{
			name: "custom separator splits the requested names",
			nodes: []EdgeRootNode{withCheckRunSuite(
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
				withCheckRunSuite(commitNode("sha1", "msg1"), checkRun("ci", string(success))),
				withCheckRunSuite(commitNode("sha2", "msg2"), checkRun("ci", "FAILURE")),
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
		// SpecificCheckPassed is never populated by hydrateCommits.
		if got[0].SpecificCheckPassed {
			t.Errorf("SpecificCheckPassed = true, want false")
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
