package main

import (
	"github-flow-manager/github"
	"os"
	"fmt"
)

func main() {
	gm := github.New(os.Getenv("GITHUB_TOKEN"))
	commits, _ := gm.GetCommits("DocPlanner", "github-flow-manager-test-repo", 10)
	firstParentCommits := github.PickFirstParentCommits(commits)

	for _, c := range firstParentCommits {
		fmt.Println(c.GetSHA(), c.GetCommit().GetMessage())
	}
}
