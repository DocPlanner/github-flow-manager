package github

import "github.com/shurcooL/githubql"

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
											Oid     githubql.String
											Message githubql.String
										}
									}
								} `graphql:"parents(first: $parentsNumber)"`
								Oid               githubql.String
								Message           githubql.String
								PushedDate        githubql.DateTime
								StatusCheckRollup struct {
									State    githubql.String
									Contexts struct {
										TotalCount githubql.Int
									} `graphql:"contexts(first: $parentsNumber)"`
								}
								CheckSuites struct {
									Nodes []struct {
										CheckRuns struct {
											Nodes []struct {
												Name       githubql.String
												Status     githubql.String
												Title      githubql.String
												Conclusion githubql.String
											}
										} `graphql:"checkRuns(first: 100)"`
									}
								} `graphql:"checkSuites(first: 10)"`
								Status struct {
									Contexts []struct {
										Context githubql.String
										State   githubql.String
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
