package flow_manager

import "github-flow-manager/github"

type evaluationResult struct {
	Commit github.Commit
	Result bool
}
