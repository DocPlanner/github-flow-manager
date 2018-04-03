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

	masterHead := firstParentCommits[0]

	err = gm.ChangeBranchHead("DocPlanner", "github-flow-manager-test-repo", "test", masterHead.SHA, false)
	if err != nil {
		fmt.Println(err)
	}


	//for _, c := range firstParentCommits {
	//	fmt.Println(c.SHA, c.Message)
	//}
}
