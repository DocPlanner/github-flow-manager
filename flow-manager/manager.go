package flow_manager

import (
	"github-flow-manager/github"
	"github.com/araddon/qlbridge/expr/builtins"
	"github.com/araddon/qlbridge/datasource"
	"github.com/araddon/qlbridge/vm"
	"github.com/araddon/qlbridge/expr"
)

func Manage(githubToken, owner, repo, sourceBranch, destinationBranch, expression string, lastCommitsNumber int, force, dryRun bool) ([]evaluationResult, error) {
	parsedExpression := expr.MustParse(expression)
	gm := github.New(githubToken)
	commits, err := gm.GetCommits(owner, repo, sourceBranch, lastCommitsNumber)
	if nil != err {
		return nil, err
	}
	firstParentCommits := github.PickFirstParentCommits(commits)

	var evaluationResultList []evaluationResult
	builtins.LoadAllBuiltins()
	for _, commit := range firstParentCommits {
		evalContext := datasource.NewContextSimpleNative(map[string]interface{}{
			"SHA":           commit.SHA,
			"Message":       commit.Message,
			"PushedDate":    commit.PushedDate,
			"StatusSuccess": commit.StatusSuccess,
		})

		val, _ := vm.Eval(evalContext, parsedExpression)
		v := val.Value()

		evaluationResultList = append(evaluationResultList, evaluationResult{Result: v.(bool), Commit: commit})

		if true == v {
			if !dryRun {
				err = gm.ChangeBranchHead(owner, repo, destinationBranch, commit.SHA, force)
				if err != nil {
					return nil, err
				}
			}
			break
		}
	}

	return evaluationResultList, nil
}
