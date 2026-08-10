package github

import "github.com/shurcooL/githubv4"

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

// CheckSuiteNode represents the information about the check suite information of the Node.
//
// Deliberately does not select the nested checkRuns connection. Check runs are read from
// statusCheckRollup.contexts instead, which returns them as a flat, de-duplicated list; nesting
// checkRuns under checkSuites multiplied the query's node count by suites × runs per commit and
// made whether a check was seen at all depend on check-suite ordering.
type CheckSuiteNode struct {
	WorkflowRun WorkflowRun
}

// CheckSuites represents the information about the check suite of a slice of Nodes
type CheckSuites struct {
	TotalCount githubv4.Int
	PageInfo   PageInfo
	Nodes      []CheckSuiteNode
}

// PageInfo carries the cursor of a connection so remaining pages can be fetched on demand.
type PageInfo struct {
	HasNextPage githubv4.Boolean
	EndCursor   githubv4.String
}

// Context represents the information about the Context
type Context struct {
	Context githubv4.String
	State   githubv4.String
}

// NodeStatus represents the information about a slice of Contexts.
//
// Commit.status.contexts is a plain list rather than a connection, so this is always the complete
// set of classic commit statuses — there is nothing to paginate.
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

// RollupStatusContext is the StatusContext member of the statusCheckRollup contexts union: a
// classic commit status.
type RollupStatusContext struct {
	Context githubv4.String
	State   githubv4.String
}

// RollupCheckRun is the CheckRun member of the statusCheckRollup contexts union.
type RollupCheckRun struct {
	Name       githubv4.String
	Status     githubv4.String
	Conclusion githubv4.String
}

// RollupContext is one entry of statusCheckRollup.contexts, a union of StatusContext and CheckRun.
// Typename says which member is populated.
type RollupContext struct {
	Typename      githubv4.String     `graphql:"__typename"`
	StatusContext RollupStatusContext `graphql:"... on StatusContext"`
	CheckRun      RollupCheckRun      `graphql:"... on CheckRun"`
}

// StatusCheckRollupContexts represents the information about the status check of rollup contexts.
//
// GitHub returns the latest run per check name here, so re-runs of the same check collapse to a
// single entry instead of appearing once per attempt.
type StatusCheckRollupContexts struct {
	TotalCount githubv4.Int
	PageInfo   PageInfo
	Nodes      []RollupContext
}

// StatusCheckRollup represents the information about the status check of rollup
type StatusCheckRollup struct {
	State    githubv4.String
	Contexts StatusCheckRollupContexts `graphql:"contexts(first: $contextsNumber)"`
}

// Author represents the information about the commit author
type Author struct {
	Name githubv4.String
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
	AuthoredDate      githubv4.DateTime
	Author            Author
	StatusCheckRollup StatusCheckRollup
	CheckSuites       CheckSuites `graphql:"checkSuites(first: $checkSuitesNumber)"`
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

// CommitSuitesQuery fetches one further page of a single commit's check suites. Used when the first
// page did not cover every suite and the commit has not yet satisfied its checks.
type CommitSuitesQuery struct {
	Repository struct {
		Object struct {
			Commit struct {
				CheckSuites CheckSuites `graphql:"checkSuites(first: $checkSuitesNumber, after: $suitesAfter)"`
			} `graphql:"... on Commit"`
		} `graphql:"object(oid: $oid)"`
	} `graphql:"repository(owner: $owner, name: $name)"`
}

// CommitContextsQuery fetches one further page of a single commit's status-check rollup contexts.
type CommitContextsQuery struct {
	Repository struct {
		Object struct {
			Commit struct {
				StatusCheckRollup struct {
					Contexts StatusCheckRollupContexts `graphql:"contexts(first: $contextsNumber, after: $contextsAfter)"`
				}
			} `graphql:"... on Commit"`
		} `graphql:"object(oid: $oid)"`
	} `graphql:"repository(owner: $owner, name: $name)"`
}
