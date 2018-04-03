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
								Oid        githubql.String
								Message    githubql.String
								PushedDate githubql.DateTime
								Status struct {
									Id    githubql.String
									State githubql.String
								}
							}
						}
					} `graphql:"history(first: $commitsNumber)"`
				} `graphql:"... on Commit"`
			}
		} `graphql:"ref(qualifiedName: $branch)"`
	} `graphql:"repository(owner: $owner, name: $name)"`
}
