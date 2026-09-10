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

	// Everything read so far that can satisfy a required name, plus what it takes to read the
	// rest: the required names, the skip policy, and a cursor per paginated connection.
	sources       checkSources
	checkNames    []string
	acceptSkipped bool
	suitesPage    PageInfo
	contextsPage  PageInfo
}

// ChecksTruncated reports whether some of this commit's check data was left unread because a
// connection had further pages. A commit that already passed needs no more data; one that did not
// may only be failing because the deciding check sits on a page nobody fetched.
func (c *Commit) ChecksTruncated() bool {
	return bool(c.suitesPage.HasNextPage) || bool(c.contextsPage.HasNextPage)
}
