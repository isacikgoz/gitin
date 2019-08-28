package git

import (
	lib "gopkg.in/libgit2/git2go.v27"
)

// CloneOptions are mostly used git clone options from a remote
type CloneOptions struct {
	Bare        bool
	Recursive   bool
	Depth       int
	Credentials Credential
}

// Clone fetches a git repository from a given url
func Clone(path string, url string, opts *CloneOptions) (*Repository, error) {
	options := &lib.CloneOptions{
		Bare: opts.Bare,
	}
	fetchOptions := &lib.FetchOptions{}
	fetchOptions.RemoteCallbacks = defaultRemoteCallbacks(opts)
	options.FetchOptions = fetchOptions
	_, err := lib.Clone(url, path, options)
	if err != nil {
		return nil, err
	}
	return Open(path)
}

func (opts *CloneOptions) authCallbackFunc(url string, uname string, credType lib.CredType) (lib.ErrorCode, *lib.Cred) {
	return defaultAuthCallback(opts, url, uname, credType)
}

func (opts *CloneOptions) certCheckCallbackFunc(cert *lib.Certificate, valid bool, hostname string) lib.ErrorCode {
	return defaultCertCheckCallback(opts, cert, valid, hostname)
}

func (opts *CloneOptions) creds() Credential {
	return opts.Credentials
}
