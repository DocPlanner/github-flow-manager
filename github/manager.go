package github

import (
	"net/http"
	"strings"

	"github.com/google/go-github/github"
	"github.com/shurcooL/githubv4"
	"golang.org/x/net/context"
	"golang.org/x/oauth2"
)

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
func (gm *Manager) GetCommits(owner, repo, branch string, lastCommitsNumber int, specificChecksNames string, sep string) ([]Commit, error) {
	if lastCommitsNumber > 100 || lastCommitsNumber < 1 {
		return nil, &Error{Message: "lastCommitsNumber must be a number between 1 and 100"} // TODO maybe in future implement pagination
	}

	q := &Query{}

	client := gm.Client
	err := client.Query(gm.Context, &q, map[string]interface{}{
		"owner":         githubv4.String(owner),
		"name":          githubv4.String(repo),
		"branch":        githubv4.String(branch),
		"commitsNumber": githubv4.Int(lastCommitsNumber),
		"parentsNumber": githubv4.Int(1),
	})
	if nil != err {
		return nil, err
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

func checkRunSet(cc int, cn string, edge Edge) int {
	for _, checkSuite := range edge.Node.CheckSuites.Nodes {
		if checkSuite.App.Name == "GitHub Actions" {
			if githubv4.String(cn) == checkSuite.WorkflowRun.Workflow.Name {
				if checkSuite.WorkflowRun.CheckSuite.Conclusion == githubv4.String(githubv4.StatusStateSuccess) {
					cc++
				}
			}
		} else {
			for _, checkRuns := range checkSuite.CheckRuns.Nodes {
				if githubv4.String(cn) == checkRuns.Name {
					if checkRuns.Conclusion == githubv4.String(githubv4.StatusStateSuccess) {
						cc++
					}
				}
			}
		}
	}
	return cc
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

		statusSuccess := false
		checkNames := strings.Split(specificChecksNames, sep)
		numChecks := len(checkNames)
		sc := 0
		cc := 0

		for _, cn := range checkNames {

			// first check if commit has commit status set
			for _, context := range edge.Node.Status.Contexts {
				if githubv4.String(cn) == context.Context {
					if context.State == githubv4.String(githubv4.StatusStateSuccess) {
						sc++
					}
				}
			}
			cc = checkRunSet(cc, cn, edge)
		}

		if numChecks == sc || numChecks == cc {
			statusSuccess = true
		}

		if specificChecksNames == "" {
			statusSuccess = edge.Node.StatusCheckRollup.State == githubv4.String(githubv4.StatusStateSuccess)
		}

		fullCommitsList = append(fullCommitsList, Commit{
			SHA:           string(edge.Node.Oid),
			Message:       string(edge.Node.Message),
			Parents:       parents,
			StatusSuccess: statusSuccess,
			PushedDate:    edge.Node.PushedDate.Time,
		})
	}

	return fullCommitsList
}
