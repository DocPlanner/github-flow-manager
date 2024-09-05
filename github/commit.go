package github

import "time"

// Commit represents a specific GitHub commit
type Commit struct {
	SHA                 string
	Message             string
	Parents             []Commit
	StatusSuccess       bool
	AuthoredDate        time.Time
	AuthorName          string
	SpecificCheckPassed bool
}
