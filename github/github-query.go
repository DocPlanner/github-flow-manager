package github

import "github.com/shurcooL/githubv4"

type CheckRunNodes struct {
	Name       githubv4.String
	Status     githubv4.String
	Title      githubv4.String
	Conclusion githubv4.String
}

type CheckRuns struct {
	Nodes []CheckRunNodes
}

type Workflow struct {
	Name githubv4.String
}

type CheckSuite struct {
	Conclusion githubv4.String
}

type WorkflowRun struct {
	Workflow   Workflow
	CheckSuite CheckSuite
}

type GithubActionApp struct {
	Name githubv4.String
}

type CheckSuiteNode struct {
	App         GithubActionApp
	WorkflowRun WorkflowRun
	CheckRuns   CheckRuns `graphql:"checkRuns(first: 25)"`
}

type CheckSuites struct {
	Nodes []CheckSuiteNode
}

type Context struct {
	Context githubv4.String
	State   githubv4.String
}

type NodeStatus struct {
	Contexts []Context
}

type EdgeNode struct {
	Oid     githubv4.String
	Message githubv4.String
}

type EdgeParent struct {
	Node EdgeNode
}

type StatusCheckRollupContexts struct {
	TotalCount githubv4.Int
}

type StatusCheckRollup struct {
	State    githubv4.String
	Contexts StatusCheckRollupContexts `graphql:"contexts(first: $parentsNumber)"`
}

type ParentsEdge struct {
	Edges []EdgeParent
}

type EdgeRootNode struct {
	Parents           ParentsEdge `graphql:"parents(first: $parentsNumber)"`
	Oid               githubv4.String
	Message           githubv4.String
	PushedDate        githubv4.DateTime
	StatusCheckRollup StatusCheckRollup
	CheckSuites       CheckSuites `graphql:"checkSuites(first: 10)"`
	Status            NodeStatus
}

type Edge struct {
	Node EdgeRootNode
}

type History struct {
	Edges []Edge
}

type TargetCommit struct {
	History History `graphql:"history(first: $commitsNumber)"`
}

type Target struct {
	Commit TargetCommit `graphql:"... on Commit"`
}

type Ref struct {
	Target Target
}

type Repository struct {
	Ref Ref `graphql:"ref(qualifiedName: $branch)"`
}

type GithubQuery struct {
	Repository Repository `graphql:"repository(owner: $owner, name: $name)"`
}
