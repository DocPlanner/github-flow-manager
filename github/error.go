package github

type Error struct {
	Message       string
	PreviousError error
}

func (e Error) Error() (string) {
	return e.Message
}
