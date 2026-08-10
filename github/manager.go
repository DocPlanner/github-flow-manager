package github

import (
	"net/http"
	"strings"

	"github.com/google/go-github/github"
	"github.com/shurcooL/githubv4"
	"golang.org/x/net/context"
	"golang.org/x/oauth2"
)

// maxPageSize is GitHub's per-connection limit for `first:`.
const maxPageSize = 100

// Manager represents the information necessary in Github to manage the repository
type Manager struct {
	Context    context.Context
	Client     *githubv4.Client
	HTTPClient *http.Client
}

// New creates a new githubManager using a github access token
func New(githubAccessToken string) *Manager {
	ctx := context.Background()
	src := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: githubAccessToken},
	)
	httpClient := oauth2.NewClient(ctx, src)
	client := githubv4.NewClient(httpClient)

	return &Manager{Context: ctx, Client: client, HTTPClient: httpClient}
}

// GetCommits recover the commits for a specific repository in a specific branch
func (gm *Manager) GetCommits(owner, repo, branch string, lastCommitsNumber int, specificChecksNames string, sep string, contextsNumber, checkSuitesNumber int) ([]Commit, error) {
	if lastCommitsNumber > maxPageSize || lastCommitsNumber < 1 {
		return nil, &Error{Message: "lastCommitsNumber must be a number between 1 and 100"} // TODO maybe in future implement pagination
	}
	if contextsNumber > maxPageSize || contextsNumber < 1 {
		return nil, &Error{Message: "contextsNumber must be a number between 1 and 100"}
	}
	if checkSuitesNumber > maxPageSize || checkSuitesNumber < 1 {
		return nil, &Error{Message: "checkSuitesNumber must be a number between 1 and 100"}
	}

	q := &Query{}

	client := gm.Client
	err := client.Query(gm.Context, &q, map[string]interface{}{
		"owner":             githubv4.String(owner),
		"name":              githubv4.String(repo),
		"branch":            githubv4.String(branch),
		"commitsNumber":     githubv4.Int(lastCommitsNumber),
		"parentsNumber":     githubv4.Int(1),
		"contextsNumber":    githubv4.Int(contextsNumber),
		"checkSuitesNumber": githubv4.Int(checkSuitesNumber),
	})
	if nil != err {
		// A repository with a lot of CI can make this query exceed GitHub's own execution time
		// limit, which surfaces as a 502 or 504 rather than as anything about the query. The
		// server cost is roughly commits × (contexts + check suites), so say which knobs shrink it.
		return nil, &Error{
			Message: "Can not read commits of " + branch + " because: " + err.Error() +
				"\nIf this is a gateway timeout, the repository has more CI than one request can" +
				" carry: lower --commits-number, or --contexts-number/--check-suites-number" +
				" (later pages are fetched on demand, so lowering them loses nothing).",
			PreviousError: err,
		}
	}

	return hydrateCommits(q, specificChecksNames, sep), nil
}

// PickFirstParentCommits recover the first parent commit of a commit history from a repository
func PickFirstParentCommits(fullCommitsList []Commit) []Commit {
	var firstParentCommits []Commit
	if len(fullCommitsList) == 0 {
		return firstParentCommits
	}

	fullCommitsMap := make(map[string]Commit)
	for _, c := range fullCommitsList {
		fullCommitsMap[c.SHA] = c
	}

	sha := fullCommitsList[0].SHA // HEAD
	for {
		c, exists := fullCommitsMap[sha]
		if !exists {
			break // last commit received from repo has a parent but parent doesn't exist in map
		}

		firstParentCommits = append(firstParentCommits, c)
		if len(c.Parents) == 0 {
			break // initial commit
		}
		sha = c.Parents[0].SHA
	}

	return firstParentCommits
}

// ChangeBranchHead change the head of a branch
// TODO remove v3 client when implemented in v4
func (gm *Manager) ChangeBranchHead(owner, repo, branch, sha string, force bool) error {
	httpClient := gm.HTTPClient

	client := github.NewClient(httpClient)
	ref, _, err := client.Git.GetRef(gm.Context, owner, repo, "heads/"+branch)
	if nil != err {
		return &Error{Message: "Can not update branch head because: " + err.Error(), PreviousError: err}
	}

	ref.GetObject().SHA = &sha

	_, _, err = client.Git.UpdateRef(gm.Context, owner, repo, ref, force)
	if nil != err {
		return &Error{Message: "Can not update branch head because: " + err.Error(), PreviousError: err}
	}

	return nil
}

// TopUpChecks reads the check data that did not fit in the first page and folds it into the
// commit's verdict.
//
// Both statusCheckRollup.contexts and checkSuites are capped at 100 entries per page, and real
// repositories exceed that. Rather than silently judging a commit on a partial view, fetch the
// remaining pages — but only when it can still change the answer, which is why callers invoke this
// per commit they actually examine rather than for the whole history up front. It stops as soon as
// every requested name has passed.
func (gm *Manager) TopUpChecks(owner, repo string, c *Commit) error {
	if len(c.checkNames) == 0 {
		return nil
	}

	for bool(c.contexts.HasNextPage) && !allPassed(c.states, c.checkNames) {
		q := &CommitContextsQuery{}
		err := gm.Client.Query(gm.Context, q, map[string]interface{}{
			"owner":          githubv4.String(owner),
			"name":           githubv4.String(repo),
			"oid":            githubv4.GitObjectID(c.SHA),
			"contextsNumber": githubv4.Int(maxPageSize),
			"contextsAfter":  githubv4.String(c.contexts.EndCursor),
		})
		if nil != err {
			return &Error{Message: "Can not read further status checks for " + c.SHA + " because: " + err.Error(), PreviousError: err}
		}

		page := q.Repository.Object.Commit.StatusCheckRollup.Contexts
		applyRollupContexts(c.states, page.Nodes)
		c.contexts = page.PageInfo
	}

	for bool(c.suites.HasNextPage) && !allPassed(c.states, c.checkNames) {
		q := &CommitSuitesQuery{}
		err := gm.Client.Query(gm.Context, q, map[string]interface{}{
			"owner":             githubv4.String(owner),
			"name":              githubv4.String(repo),
			"oid":               githubv4.GitObjectID(c.SHA),
			"checkSuitesNumber": githubv4.Int(maxPageSize),
			"suitesAfter":       githubv4.String(c.suites.EndCursor),
		})
		if nil != err {
			return &Error{Message: "Can not read further check suites for " + c.SHA + " because: " + err.Error(), PreviousError: err}
		}

		page := q.Repository.Object.Commit.CheckSuites
		applyCheckSuites(c.states, page.Nodes)
		c.suites = page.PageInfo
	}

	c.settle()

	return nil
}

// promote raises a name's state, never lowers it. A check that succeeded stays succeeded: the same
// name legitimately appears more than once — as both a classic status and a check run, or as
// several re-runs — and the commit should be judged on the best of them rather than on how many
// times it was seen.
func promote(states map[string]CheckState, name string, state CheckState) {
	if current, ok := states[name]; !ok || state > current {
		states[name] = state
	}
}

// applyClassicStatuses folds in the commit's classic commit statuses.
func applyClassicStatuses(states map[string]CheckState, contexts []Context) {
	for _, ctx := range contexts {
		name := string(ctx.Context)
		if _, requested := states[name]; !requested {
			continue
		}

		if ctx.State == githubv4.String(githubv4.StatusStateSuccess) {
			promote(states, name, CheckPassed)
			continue
		}
		promote(states, name, CheckFailed)
	}
}

// applyRollupContexts folds in the status-check rollup, which carries both classic statuses and
// check runs, latest-per-name.
func applyRollupContexts(states map[string]CheckState, nodes []RollupContext) {
	for _, node := range nodes {
		switch node.Typename {
		case "StatusContext":
			name := string(node.StatusContext.Context)
			if _, requested := states[name]; !requested {
				continue
			}

			if node.StatusContext.State == githubv4.String(githubv4.StatusStateSuccess) {
				promote(states, name, CheckPassed)
				continue
			}
			promote(states, name, CheckFailed)

		case "CheckRun":
			name := string(node.CheckRun.Name)
			if _, requested := states[name]; !requested {
				continue
			}

			if node.CheckRun.Conclusion == githubv4.String(githubv4.StatusStateSuccess) {
				promote(states, name, CheckPassed)
				continue
			}
			if node.CheckRun.Status != githubv4.String(githubv4.CheckStatusStateCompleted) {
				promote(states, name, CheckPending)
				continue
			}
			promote(states, name, CheckFailed)
		}
	}
}

// applyCheckSuites folds in whole-workflow results: a requested name may be the name of a GitHub
// Actions workflow rather than of an individual check, in which case the suite's conclusion decides.
func applyCheckSuites(states map[string]CheckState, nodes []CheckSuiteNode) {
	for _, suite := range nodes {
		if (suite.WorkflowRun == WorkflowRun{}) {
			continue
		}

		name := string(suite.WorkflowRun.Workflow.Name)
		if _, requested := states[name]; !requested {
			continue
		}

		if suite.WorkflowRun.CheckSuite.Conclusion == githubv4.String(githubv4.StatusStateSuccess) {
			promote(states, name, CheckPassed)
			continue
		}
		promote(states, name, CheckFailed)
	}
}

// allPassed reports whether every requested name has been satisfied.
func allPassed(states map[string]CheckState, names []string) bool {
	for _, name := range names {
		if states[name] != CheckPassed {
			return false
		}
	}
	return true
}

// settle publishes the accumulated per-name states as the commit's verdict. Safe to call again
// after more pages are read.
func (c *Commit) settle() {
	c.CheckResults = make([]CheckResult, 0, len(c.checkNames))
	for _, name := range c.checkNames {
		c.CheckResults = append(c.CheckResults, CheckResult{Name: name, State: c.states[name]})
	}
	c.StatusSuccess = allPassed(c.states, c.checkNames)
}

func hydrateCommits(q *Query, specificChecksNames string, sep string) []Commit {

	var fullCommitsList []Commit
	for _, edge := range q.Repository.Ref.Target.Commit.History.Edges {
		var parents []Commit
		for _, parent := range edge.Node.Parents.Edges {
			parents = append(parents, Commit{
				SHA:     string(parent.Node.Oid),
				Message: string(parent.Node.Message),
			})
		}

		commit := Commit{
			SHA:          string(edge.Node.Oid),
			Message:      string(edge.Node.Message),
			Parents:      parents,
			AuthoredDate: edge.Node.AuthoredDate.Time,
			AuthorName:   string(edge.Node.Author.Name),
			suites:       edge.Node.CheckSuites.PageInfo,
			contexts:     edge.Node.StatusCheckRollup.Contexts.PageInfo,
		}

		if specificChecksNames == "" {
			// No names requested: defer to GitHub's own aggregate verdict for the commit.
			commit.StatusSuccess = edge.Node.StatusCheckRollup.State == githubv4.String(githubv4.StatusStateSuccess)
			fullCommitsList = append(fullCommitsList, commit)
			continue
		}

		commit.checkNames = strings.Split(specificChecksNames, sep)
		commit.states = make(map[string]CheckState, len(commit.checkNames))
		for _, name := range commit.checkNames {
			commit.states[name] = CheckNotFound
		}

		applyClassicStatuses(commit.states, edge.Node.Status.Contexts)
		applyRollupContexts(commit.states, edge.Node.StatusCheckRollup.Contexts.Nodes)
		applyCheckSuites(commit.states, edge.Node.CheckSuites.Nodes)
		commit.settle()

		fullCommitsList = append(fullCommitsList, commit)
	}

	return fullCommitsList
}
