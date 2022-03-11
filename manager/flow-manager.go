package manager

import (
	"fmt"

	"github.com/Docplanner/github-flow-manager/github"
	"github.com/araddon/qlbridge/datasource"
	"github.com/araddon/qlbridge/expr"
	"github.com/araddon/qlbridge/expr/builtins"
	"github.com/araddon/qlbridge/vm"
)

func checkGithubToken(githubToken string) error {
	if githubToken == "" {
		return fmt.Errorf("github token not set")
	}
	return nil
}

// Manage will do the necessary actions to move the head from one branch to another
func Manage(githubToken, owner, repo, sourceBranch, destinationBranch, expression string, specificChecksNames string, sep string, lastCommitsNumber int, force, dryRun bool) ([]EvaluationResult, error) {
	err := checkGithubToken(githubToken)
	if err != nil {
		return nil, err
	}
	parsedExpression := expr.MustParse(expression)
	gm := github.New(githubToken)
	commits, err := gm.GetCommits(owner, repo, sourceBranch, lastCommitsNumber, specificChecksNames, sep)
	if nil != err {
		return nil, err
	}
	firstParentCommits := github.PickFirstParentCommits(commits)

	destinationCommits, err := gm.GetCommits(owner, repo, destinationBranch, 1, specificChecksNames, sep)
	if nil != err {
		return nil, err
	}

	var evaluationResultList []EvaluationResult
	builtins.LoadAllBuiltins()
	for _, commit := range firstParentCommits {

		if destinationCommits[0].SHA == commit.SHA {
			fmt.Println("COMMIT ID: " + commit.SHA + " IS ALREADY IN " + destinationBranch + " branch. EXITING THE PROCESS WITHOUT ANY ACTION.")
			return evaluationResultList, nil
		}

		evalContext := datasource.NewContextSimpleNative(map[string]interface{}{
			"SHA":           commit.SHA,
			"Message":       commit.Message,
			"PushedDate":    commit.PushedDate,
			"StatusSuccess": commit.StatusSuccess,
		})

		val, _ := vm.Eval(evalContext, parsedExpression)
		v := val.Value()

		evaluationResultList = append(evaluationResultList, EvaluationResult{Result: v.(bool), Commit: commit})

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
