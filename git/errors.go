package git

// Error is the errors from the git package
type Error string

func (e Error) Error() string {
	return string(e)
}

var (
	// ErrCannotOpenRepo is returned when the repo couldn't be loaded from filesystem
	ErrCannotOpenRepo Error = "cannot load repository"
)
