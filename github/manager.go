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

func (gm *githubManager) GetCommits(owner, repo string, lastCommitsNumber int) ([]*github.RepositoryCommit, error) {
	client := gm.Client
	ctx := gm.Context

	var fullCommitsList []*github.RepositoryCommit
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

	return fullCommitsList, nil
}

func PickFirstParentCommits(fullCommitsList []*github.RepositoryCommit) ([]*github.RepositoryCommit) {
	var firstParentCommits []*github.RepositoryCommit
	if 0 == len(fullCommitsList) {
		return firstParentCommits
	}

	fullCommitsMap := make(map[string]*github.RepositoryCommit)
	for _, c := range fullCommitsList {
		fullCommitsMap[c.GetSHA()] = c
	}

	sha := fullCommitsList[0].GetSHA() // HEAD
	for {
		c, exists := fullCommitsMap[sha]
		if !exists {
			break // last commit received from repo has a parent but parent doesnt exist in map
		}

		firstParentCommits = append(firstParentCommits, c)
		if 0 == len(c.Parents) {
			break // initial commit
		}
		sha = c.Parents[0].GetSHA()
	}

	return firstParentCommits
}