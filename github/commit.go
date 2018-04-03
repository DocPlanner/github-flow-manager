package github

import "time"

type Commit struct {
	SHA           string
	Message       string
	Parents       []Commit
	StatusSuccess bool
	PushedDate    time.Time
}
