package github

import "github.com/shurcooL/githubv4"

type githubQuery struct {
	Repository struct {
		Ref struct {
			Target struct {
				Commit struct {
					History struct {
						Edges []struct {
							Node struct {
								Parents struct {
									Edges []struct {
										Node struct {
											Oid     githubv4.String
											Message githubv4.String
										}
									}
								} `graphql:"parents(first: $parentsNumber)"`
								Oid               githubv4.String
								Message           githubv4.String
								PushedDate        githubv4.DateTime
								StatusCheckRollup struct {
									State    githubv4.String
									Contexts struct {
										TotalCount githubv4.Int
									} `graphql:"contexts(first: $parentsNumber)"`
								}
								CheckSuites struct {
									Nodes []struct {
										CheckRuns struct {
											Nodes []struct {
												Name       githubv4.String
												Status     githubv4.String
												Title      githubv4.String
												Conclusion githubv4.String
											}
										} `graphql:"checkRuns(first: 10)"`
									}
								} `graphql:"checkSuites(first: 10)"`
								Status struct {
									Contexts []struct {
										Context githubv4.String
										State   githubv4.String
									}
								}
							}
						}
					} `graphql:"history(first: $commitsNumber)"`
				} `graphql:"... on Commit"`
			}
		} `graphql:"ref(qualifiedName: $branch)"`
	} `graphql:"repository(owner: $owner, name: $name)"`
}
