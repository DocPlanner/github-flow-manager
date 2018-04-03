package github

import (
	"golang.org/x/oauth2"
	"golang.org/x/net/context"
	"github.com/shurcooL/githubql"
)

type githubManager struct {
	Context context.Context
	Client  *githubql.Client
}

func New(githubAccessToken string) (*githubManager) {
	ctx := context.Background()
	src := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: githubAccessToken},
	)
	httpClient := oauth2.NewClient(ctx, src)
	client := githubql.NewClient(httpClient)

	return &githubManager{Context: ctx, Client: client}
}

func (gm *githubManager) GetCommits(owner, repo, branch string, lastCommitsNumber int) ([]Commit, error) {
	if lastCommitsNumber > 100 || lastCommitsNumber < 1 {
		return nil, &Error{Message: "lastCommitsNumber must be a number between 1 and 100"} // TODO maybe in future implement pagination
	}

	q := &githubQuery{}

	client := gm.Client
	err := client.Query(gm.Context, &q, map[string]interface{}{
		"owner":         githubql.String(owner),
		"name":          githubql.String(repo),
		"branch":        githubql.String(branch),
		"commitsNumber": githubql.Int(lastCommitsNumber),
		"parentsNumber": githubql.Int(1),
	})
	if nil != err {
		return nil, err
	}

	return hydrateCommits(q), nil
}

func PickFirstParentCommits(fullCommitsList []Commit) ([]Commit) {
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

func hydrateCommits(q *githubQuery) ([]Commit) {
	var fullCommitsList []Commit
	for _, edge := range q.Repository.Ref.Target.Commit.History.Edges {
		var parents []Commit
		for _, parent := range edge.Node.Parents.Edges {
			parents = append(parents, Commit{
				SHA:     string(parent.Node.Oid),
				Message: string(parent.Node.Message),
			})
		}
		fullCommitsList = append(fullCommitsList, Commit{
			SHA:           string(edge.Node.Oid),
			Message:       string(edge.Node.Message),
			Parents:       parents,
			StatusSuccess: bool(edge.Node.Status.State == githubql.String(githubql.StatusStateSuccess)),
		})
	}

	return fullCommitsList
}
