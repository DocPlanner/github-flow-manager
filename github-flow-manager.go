package main

import (
	"github-flow-manager/github"
	"os"
)

func main() {
	gm := github.New(os.Getenv("GITHUB_TOKEN"))
	gm.GetFirstParentCommits("DocPlanner", "github-flow-manager-test-repo", 1)
}
