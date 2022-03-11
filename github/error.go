package github

// Error represents a specific error in commit check
type Error struct {
	Message       string
	PreviousError error
}

func (e Error) Error() string {
	return e.Message
}
