package github

import (
	"golang.org/x/oauth2"
	"github.com/google/go-github/github"
	"golang.org/x/net/context"
)

type githubManager struct {
	Context context.Context
	Client  *github.Client
}

func New(githubAccessToken string) (*githubManager) {
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: githubAccessToken},
	)
	tc := oauth2.NewClient(ctx, ts)

	client := github.NewClient(tc)

	return &githubManager{Context: ctx, Client: client}
}

func (gm *githubManager) GetFirstParentCommits(owner, repo string, lastCommitsNumber int) ([]*github.RepositoryCommit, error) {
	client := gm.Client
	ctx := gm.Context

	var fullCommitsList []*github.RepositoryCommit;
	commitsLeft := lastCommitsNumber
	pageNumber := 1
	for commitsLeft > 0 {
		commitsAmountToGetThisTime := commitsLeft
		if commitsLeft > 100 {
			commitsAmountToGetThisTime = 100
		}

		commits, _, err := client.Repositories.ListCommits(ctx, owner, repo, &github.CommitsListOptions{ListOptions: github.ListOptions{Page: pageNumber, PerPage: commitsAmountToGetThisTime}})
		if nil != err {
			return nil, err
		}

		for _, element := range commits {
			fullCommitsList = append(fullCommitsList, element)
		}

		pageNumber++
		commitsLeft -= commitsAmountToGetThisTime
	}

	// TODO: remove commits which are pointed by some parents and its parents and its parents... -- basicly implement --first-parent logic here from git log command

	return fullCommitsList, nil
}
