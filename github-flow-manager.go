package main

import (
	"github-flow-manager/github"
	"os"
	"fmt"
)

func main() {
	gm := github.New(os.Getenv("GITHUB_TOKEN"))
	commits, err := gm.GetCommits("DocPlanner", "github-flow-manager-test-repo", "master", 10)
	if nil != err {
		fmt.Println(err)
		panic("")
	}
	firstParentCommits := github.PickFirstParentCommits(commits)

	for _, c := range firstParentCommits {
		fmt.Println(c.SHA, c.Message)
	}
}
