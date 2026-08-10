package github

import "time"

// CheckState is the outcome of resolving one requested status check name against a commit.
type CheckState int

const (
	// CheckNotFound means no status, check run or workflow of that name exists on the commit.
	// Kept distinct from CheckFailed because the two call for opposite fixes: a wrong name in
	// configuration versus a genuinely red build.
	CheckNotFound CheckState = iota
	// CheckPending means a check run of that name exists but has not completed.
	CheckPending
	// CheckFailed means it completed with any conclusion other than SUCCESS.
	CheckFailed
	// CheckPassed means at least one instance concluded SUCCESS.
	CheckPassed
)

// String renders the state for the verbose commit table.
func (s CheckState) String() string {
	switch s {
	case CheckPassed:
		return "passed"
	case CheckFailed:
		return "failed"
	case CheckPending:
		return "pending"
	default:
		return "not found"
	}
}

// CheckResult is how one requested status check name resolved for a commit. Reported so that a
// commit which fails evaluation says why, rather than only that it failed.
type CheckResult struct {
	Name  string
	State CheckState
}

// Commit represents a specific GitHub commit
type Commit struct {
	SHA           string
	Message       string
	Parents       []Commit
	StatusSuccess bool
	AuthoredDate  time.Time
	AuthorName    string
	CheckResults  []CheckResult

	// checkNames are the names this commit was asked to satisfy, in the order given.
	checkNames []string
	// states accumulates the best-known state per name across however many pages were read.
	states map[string]CheckState

	// Cursors for the connections that GitHub caps at 100 per page. Non-empty HasNextPage means
	// the commit was only partially read and TopUpChecks can fetch the rest.
	suites   PageInfo
	contexts PageInfo
}

// ChecksTruncated reports whether some of this commit's check data was left unread because a
// connection had more pages. A commit that already passed needs no more data; one that did not may
// only be failing because the deciding check was on a page nobody fetched.
func (c *Commit) ChecksTruncated() bool {
	return bool(c.suites.HasNextPage) || bool(c.contexts.HasNextPage)
}
