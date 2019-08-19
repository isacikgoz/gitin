package git

import (
	lib "github.com/libgit2/git2go"
)

// FetchOptions provides common options for fetch command
type FetchOptions struct {
	Remote      string
	Credentials Credential
	Prune       bool
	All         bool
	Tags        bool
}

// Fetch downloads refs from given remote
func (r *Repository) Fetch(opts *FetchOptions) error {
	rc := r.essence.Remotes
	// refscpes := []string{}
	options := &lib.FetchOptions{}
	options.RemoteCallbacks = defaultRemoteCallbacks(opts)
	if opts.Tags {
		options.DownloadTags = lib.DownloadTagsAll
	}
	if opts.Prune {
		options.Prune = lib.FetchPruneOn
	}
	if opts.All {

	} else {
		if len(opts.Remote) <= 0 {
			return ErrNoRemoteName
		}
		remote, err := rc.Lookup(opts.Remote)
		if err != nil {
			return ErrNotValidRemoteName
		}
		if err := remote.Fetch(nil, options, ""); err != nil {
			return err
		}
	}
	return nil
}

func (opts *FetchOptions) authCallbackFunc(url string, uname string, credType lib.CredType) (lib.ErrorCode, *lib.Cred) {
	return defaultAuthCallback(opts, url, uname, credType)
}

func (opts *FetchOptions) certCheckCallbackFunc(cert *lib.Certificate, valid bool, hostname string) lib.ErrorCode {
	return defaultCertCheckCallback(opts, cert, valid, hostname)
}

func (opts *FetchOptions) creds() Credential {
	return opts.Credentials
}
