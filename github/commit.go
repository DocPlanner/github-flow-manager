package github

type Commit struct {
	SHA           string
	Message       string
	Parents       []Commit
	StatusSuccess bool
}
