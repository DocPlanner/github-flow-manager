package github

import "github.com/shurcooL/githubv4"

// CheckRunNodes represents the checkRun status of the Nodes
type CheckRunNodes struct {
	Name       githubv4.String
	Status     githubv4.String
	Title      githubv4.String
	Conclusion githubv4.String
}

// CheckRuns represents the check run of an array of Nodes
type CheckRuns struct {
	Nodes []CheckRunNodes
}

// Workflow represents the information of the Github Workflow
type Workflow struct {
	Name githubv4.String
}

// CheckSuite represents the information about the check suite obtained from Github
type CheckSuite struct {
	Conclusion githubv4.String
}

// WorkflowRun represents the information about the workflow run execution
type WorkflowRun struct {
	Workflow   Workflow
	CheckSuite CheckSuite
}

// CheckSuiteNode represents the information about the check suite information of the Node
type CheckSuiteNode struct {
	WorkflowRun WorkflowRun
	CheckRuns   CheckRuns `graphql:"checkRuns(first: 25)"`
}

// CheckSuites represents the information about the check suite of a slice of Nodes
type CheckSuites struct {
	Nodes []CheckSuiteNode
}

// Context represents the information about the Context
type Context struct {
	Context githubv4.String
	State   githubv4.String
}

// NodeStatus represents the information about a slice of Contexts
type NodeStatus struct {
	Contexts []Context
}

// EdgeNode represents the information about an edge node
type EdgeNode struct {
	Oid     githubv4.String
	Message githubv4.String
}

// EdgeParent represents the information about a parent node
type EdgeParent struct {
	Node EdgeNode
}

// StatusCheckRollupContexts represents the information about the status check of roullup contexts
type StatusCheckRollupContexts struct {
	TotalCount githubv4.Int
}

// StatusCheckRollup represents the information about the status check of rollup
type StatusCheckRollup struct {
	State    githubv4.String
	Contexts StatusCheckRollupContexts `graphql:"contexts(first: $parentsNumber)"`
}

// ParentsEdge represents the information about the parents edge
type ParentsEdge struct {
	Edges []EdgeParent
}

// EdgeRootNode represents the information about a edge root node
type EdgeRootNode struct {
	Parents           ParentsEdge `graphql:"parents(first: $parentsNumber)"`
	Oid               githubv4.String
	Message           githubv4.String
	PushedDate        githubv4.DateTime
	StatusCheckRollup StatusCheckRollup
	CheckSuites       CheckSuites `graphql:"checkSuites(first: 20)"`
	Status            NodeStatus
}

// Edge represents the information about a edge element
type Edge struct {
	Node EdgeRootNode
}

// History represents the information about a slice of Edges
type History struct {
	Edges []Edge
}

// TargetCommit represents the information about a commit history
type TargetCommit struct {
	History History `graphql:"history(first: $commitsNumber)"`
}

// Target represents the target of a specific commit
type Target struct {
	Commit TargetCommit `graphql:"... on Commit"`
}

// Ref represents the information about a ref element
type Ref struct {
	Target Target
}

// Repository represents the information obtained about a repository in a Github Query
type Repository struct {
	Ref Ref `graphql:"ref(qualifiedName: $branch)"`
}

// Query represents the information obtained in a Github Query
type Query struct {
	Repository Repository `graphql:"repository(owner: $owner, name: $name)"`
}
