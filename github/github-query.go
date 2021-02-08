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
										Nodes []struct {
											//Cursor   githubql.String
											CheckRun struct {
												Name       githubql.String
												Conclusion githubql.String
											} `graphql:"... on CheckRun"`
										}
									} `graphql:"contexts(first: $parentsNumber)"`
								}
							}
						}
					} `graphql:"history(first: $commitsNumber)"`
				} `graphql:"... on Commit"`
			}
		} `graphql:"ref(qualifiedName: $branch)"`
	} `graphql:"repository(owner: $owner, name: $name)"`
}
