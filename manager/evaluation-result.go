package manager

import "github.com/Docplanner/github-flow-manager/github"

// EvaluationResult represents the evaluation result of a specific commit
type EvaluationResult struct {
	Commit github.Commit
	Result bool
}
