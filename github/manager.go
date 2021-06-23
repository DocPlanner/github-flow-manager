package github

import (
	"net/http"
	"strings"

	"github.com/google/go-github/github"
	"github.com/shurcooL/githubv4"
	"golang.org/x/net/context"
	"golang.org/x/oauth2"
)

type githubManager struct {
	Context    context.Context
	Client     *githubv4.Client
	HttpClient *http.Client
}

func New(githubAccessToken string) *githubManager {
	ctx := context.Background()
	src := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: githubAccessToken},
	)
	httpClient := oauth2.NewClient(ctx, src)
	client := githubv4.NewClient(httpClient)

	return &githubManager{Context: ctx, Client: client, HttpClient: httpClient}
}

func (gm *githubManager) GetCommits(owner, repo, branch string, lastCommitsNumber int, specificChecksNames string, sep string) ([]Commit, error) {
	if lastCommitsNumber > 100 || lastCommitsNumber < 1 {
		return nil, &Error{Message: "lastCommitsNumber must be a number between 1 and 100"} // TODO maybe in future implement pagination
	}

	q := &githubQuery{}

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

func PickFirstParentCommits(fullCommitsList []Commit) []Commit {
	var firstParentCommits []Commit
	if 0 == len(fullCommitsList) {
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
		if 0 == len(c.Parents) {
			break // initial commit
		}
		sha = c.Parents[0].SHA
	}

	return firstParentCommits
}

// TODO remove v3 client when implemented in v4
func (gm *githubManager) ChangeBranchHead(owner, repo, branch, sha string, force bool) error {
	httpClient := gm.HttpClient

	client := github.NewClient(httpClient)
	ref, _, err := client.Git.GetRef(gm.Context, owner, repo, "heads/"+branch)
	if nil != err {
		return &Error{Message: "Can not update branch head because: " + err.Error(), PreviousError: err}
	}

	ref.GetObject().SHA = &sha

	ref, _, err = client.Git.UpdateRef(gm.Context, owner, repo, ref, force)
	if nil != err {
		return &Error{Message: "Can not update branch head because: " + err.Error(), PreviousError: err}
	}

	return nil
}

func hydrateCommits(q *githubQuery, specificChecksNames string, sep string) []Commit {
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

			// then check  if commit has check-run set
			for _, checkSuite := range edge.Node.CheckSuites.Nodes {
				for _, checkRuns := range checkSuite.CheckRuns.Nodes {
					if githubv4.String(cn) == checkRuns.Name {
						if checkRuns.Conclusion == githubv4.String(githubv4.StatusStateSuccess) {
							cc++
						}
					}
				}
			}
		}

		if numChecks == sc || numChecks == cc {
			statusSuccess = true
		}

		if numChecks == 0 {
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
