package github

import (
	"net/http"

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

// GetCommits recover the commits for a specific repository in a specific branch.
// When specificChecksNames is set, every name it contains must be satisfied on a
// commit for its StatusSuccess to be true; acceptSkippedChecks additionally lets
// a check GitHub reports as SKIPPED count as satisfied.
func (gm *Manager) GetCommits(owner, repo, branch string, lastCommitsNumber int, specificChecksNames string, sep string, acceptSkippedChecks bool, contextsNumber, checkSuitesNumber int) ([]Commit, error) {
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
		// limit, which comes back as a 502 or 504 saying nothing about the query. The server-side
		// work is roughly commits × (contexts + check suites), so name the knobs that shrink it.
		return nil, &Error{
			Message: "Can not read commits of " + branch + " because: " + err.Error() +
				"\nIf this is a gateway timeout, the repository has more CI than one request can" +
				" carry: lower --commits-number, or --contexts-number/--check-suites-number" +
				" (later pages are fetched on demand, so lowering them loses nothing).",
			PreviousError: err,
		}
	}

	return hydrateCommits(q, specificChecksNames, sep, acceptSkippedChecks), nil
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
// Both statusCheckRollup.contexts and checkSuites are capped at 100 entries per page and real
// repositories exceed that, so a commit can look unsatisfied purely because the deciding check sat
// on a page nobody asked for. Rather than judging on a partial view, fetch the rest — but only when
// it can still change the answer, which is why callers invoke this per commit they actually examine
// instead of for the whole history up front. It stops as soon as every required name is satisfied.
func (gm *Manager) TopUpChecks(owner, repo string, c *Commit) error {
	if len(c.checkNames) == 0 {
		return nil
	}

	for bool(c.contextsPage.HasNextPage) && !c.StatusSuccess {
		q := &CommitContextsQuery{}
		err := gm.Client.Query(gm.Context, q, map[string]interface{}{
			"owner":          githubv4.String(owner),
			"name":           githubv4.String(repo),
			"oid":            githubv4.GitObjectID(c.SHA),
			"contextsNumber": githubv4.Int(maxPageSize),
			"contextsAfter":  githubv4.String(c.contextsPage.EndCursor),
		})
		if nil != err {
			return &Error{Message: "Can not read further status checks for " + c.SHA + " because: " + err.Error(), PreviousError: err}
		}

		page := q.Repository.Object.Commit.StatusCheckRollup.Contexts
		c.sources.rollupContexts = append(c.sources.rollupContexts, page.Nodes...)
		c.contextsPage = page.PageInfo
		c.reevaluate()
	}

	for bool(c.suitesPage.HasNextPage) && !c.StatusSuccess {
		q := &CommitSuitesQuery{}
		err := gm.Client.Query(gm.Context, q, map[string]interface{}{
			"owner":             githubv4.String(owner),
			"name":              githubv4.String(repo),
			"oid":               githubv4.GitObjectID(c.SHA),
			"checkSuitesNumber": githubv4.Int(maxPageSize),
			"suitesAfter":       githubv4.String(c.suitesPage.EndCursor),
		})
		if nil != err {
			return &Error{Message: "Can not read further check suites for " + c.SHA + " because: " + err.Error(), PreviousError: err}
		}

		page := q.Repository.Object.Commit.CheckSuites
		c.sources.suites = append(c.sources.suites, page.Nodes...)
		c.suitesPage = page.PageInfo
		c.reevaluate()
	}

	return nil
}

// reevaluate re-runs the per-name evaluation over everything read so far.
func (c *Commit) reevaluate() {
	c.StatusSuccess, c.ChecksSummary = evaluateSpecificChecks(c.sources, c.checkNames, c.acceptSkipped)
}

func hydrateCommits(q *Query, specificChecksNames string, sep string, acceptSkippedChecks bool) []Commit {

	var fullCommitsList []Commit
	for _, edge := range q.Repository.Ref.Target.Commit.History.Edges {
		var parents []Commit
		for _, parent := range edge.Node.Parents.Edges {
			parents = append(parents, Commit{
				SHA:     string(parent.Node.Oid),
				Message: string(parent.Node.Message),
			})
		}

		var statusSuccess bool
		var checksSummary []string
		var checkNames []string
		var sources checkSources

		if specificChecksNames == "" {
			// No specific names were asked for, so trust GitHub's own rollup.
			statusSuccess = edge.Node.StatusCheckRollup.State == githubv4.String(githubv4.StatusStateSuccess)
		} else {
			checkNames = splitCheckNames(specificChecksNames, sep)
			sources = sourcesFromEdge(edge)
			statusSuccess, checksSummary = evaluateSpecificChecks(sources, checkNames, acceptSkippedChecks)
		}

		fullCommitsList = append(fullCommitsList, Commit{
			SHA:           string(edge.Node.Oid),
			Message:       string(edge.Node.Message),
			Parents:       parents,
			StatusSuccess: statusSuccess,
			ChecksSummary: checksSummary,
			AuthoredDate:  edge.Node.AuthoredDate.Time,
			AuthorName:    string(edge.Node.Author.Name),
			sources:       sources,
			checkNames:    checkNames,
			acceptSkipped: acceptSkippedChecks,
			suitesPage:    edge.Node.CheckSuites.PageInfo,
			contextsPage:  edge.Node.StatusCheckRollup.Contexts.PageInfo,
		})
	}

	return fullCommitsList
}
