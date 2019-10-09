package git

// Error is the errors from the git package
type Error string

func (e Error) Error() string {
	return string(e)
}

var (
	// ErrAuthenticationRequired as the name implies
	ErrAuthenticationRequired Error = "authentication required"
	// ErrAuthenticationType means that given credentials cannot be used for given repository url
	ErrAuthenticationType Error = "authentication method is not valid"
	// ErrClone is a generic clone error
	ErrClone Error = "cannot clone repo"
	// ErrCannotOpenRepo is returned when the repo couldn't be loaded from filesystem
	ErrCannotOpenRepo Error = "cannot load repository"
	// ErrCreateCallbackFail is returned when an error occurred while creating callbacks
	ErrCreateCallbackFail Error = "cannot create default callbacks"
	// ErrNoRemoteName if the remote name is empty while fetching
	ErrNoRemoteName Error = "remote name not specified"
	// ErrNotValidRemoteName is returned if the given remote name is not found
	ErrNotValidRemoteName Error = "not a valid remote name"
	// ErrAlreadyUpToDate if the repo is up-to-date
	ErrAlreadyUpToDate Error = "already up-to-date"
	// ErrFastForwardOnly if the merge can be made by fast-forward
	ErrFastForwardOnly Error = "fast-forward only"
	// ErrBranchNotFound is returned when the given ref can't found
	ErrBranchNotFound Error = "cannot locate remote-tracking branch"
	// ErrEntryNotIndexed is returned when the entry is not indexed
	ErrEntryNotIndexed Error = "entry is not indexed"
)
