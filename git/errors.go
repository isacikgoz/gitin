package git

import (
	"errors"
)

var (
	// ErrAuthenticationRequired as the name implies
	ErrAuthenticationRequired = errors.New("authentication required")
	// ErrAuthenticationType means that given credentials cannot be used for given repository url
	ErrAuthenticationType = errors.New("authentication method is not valid")
	// ErrClone is a generic clone error
	ErrClone = errors.New("cannot clone repo")
	// ErrCannotOpenRepo is returned when the repo couldn't be loaded from filesystem
	ErrCannotOpenRepo = errors.New("cannot load repository")
	// ErrCreateCallbackFail is reuterned when an error occurred while creating callbacks
	ErrCreateCallbackFail = errors.New("cannot create default callbacks")
	// ErrNoRemoteName if the remote name is empty while fetching
	ErrNoRemoteName = errors.New("remote name not specified")
	// ErrNotValidRemoteName is returned if the given remote name is not found
	ErrNotValidRemoteName = errors.New("not a valid remote name")
	// ErrAlreadyUpToDate if the repo is up-to-date
	ErrAlreadyUpToDate = errors.New("already up-to-date")
	// ErrFastForwardOnly if the merge can be made by fast-forward
	ErrFastForwardOnly = errors.New("fast-forward only")
	// ErrBranchNotFound is returned when the given ref can't found
	ErrBranchNotFound = errors.New("cannot locate remote-tracking branch")
	// ErrEntryNotIndexed is returned when the entry is not indexed
	ErrEntryNotIndexed = errors.New("entry is not indexed")
)
