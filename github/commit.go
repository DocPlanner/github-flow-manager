package github

import "time"

// Commit represents a specific GitHub commit
type Commit struct {
	SHA           string
	Message       string
	Parents       []Commit
	StatusSuccess bool
	// ChecksSummary explains, one line per required check name, why
	// StatusSuccess came out the way it did. Empty when no specific check names
	// were requested and the GitHub status rollup was used instead.
	ChecksSummary       []string
	AuthoredDate        time.Time
	AuthorName          string
	SpecificCheckPassed bool
}
