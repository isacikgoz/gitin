package git

import (
	lib "gopkg.in/libgit2/git2go.v27"
)

// CredType defines the credentials type for authentication with remote
type CredType uint8

const (
	// CredTypeUserpassPlaintext is used for http, https schemes
	CredTypeUserpassPlaintext CredType = iota
	// CredTypeSSHKey is for authenticating over ssh
	CredTypeSSHKey
	// CredTypeSSHAgent is used when using an agent for ssh auth
	CredTypeSSHAgent
)

// Credential is an interface for specfying its type
type Credential interface {
	// Type returns the type of credential
	Type() CredType
}

// OptionsWithCreds provides an interface to get fetch callbacks
type OptionsWithCreds interface {
	authCallbackFunc(string, string, lib.CredType) (lib.ErrorCode, *lib.Cred)
	certCheckCallbackFunc(*lib.Certificate, bool, string) lib.ErrorCode
	creds() Credential
}

// CredentialsAsPlainText contains basic username and password information
type CredentialsAsPlainText struct {
	UserName string
	Password string
}

// Type returns the type of credential
func (c *CredentialsAsPlainText) Type() CredType {
	return CredTypeUserpassPlaintext
}

// CredentialsAsSSHKey contains ssh file paths and related information
type CredentialsAsSSHKey struct {
	UserName       string
	PublicKeyPath  string
	PrivateKeyPath string
	Passphrase     string
}

// Type returns the type of credential
func (c *CredentialsAsSSHKey) Type() CredType {
	return CredTypeSSHKey
}

// CredentialsAsSSHAgent holds only usernmae if ssh daemon working
type CredentialsAsSSHAgent struct {
	UserName string
}

// Type returns the type of credential
func (c *CredentialsAsSSHAgent) Type() CredType {
	return CredTypeSSHAgent
}

func defaultRemoteCallbacks(opts OptionsWithCreds) lib.RemoteCallbacks {
	rcb := lib.RemoteCallbacks{}
	rcb.CredentialsCallback = opts.authCallbackFunc
	rcb.CertificateCheckCallback = opts.certCheckCallbackFunc
	return rcb
}

func defaultAuthCallback(opts OptionsWithCreds, url string, uname string, credType lib.CredType) (lib.ErrorCode, *lib.Cred) {
	if opts.creds() == nil {
		return lib.ErrAuth, nil
	}
	cr := opts.creds()

	switch credType {
	case lib.CredTypeUserpassPlaintext:
		switch cr.(type) {
		case *CredentialsAsPlainText:
			credentials := cr.(*CredentialsAsPlainText)
			errCode, cred := lib.NewCredUserpassPlaintext(credentials.UserName, credentials.Password)
			return lib.ErrorCode(errCode), &cred
		default:
			return lib.ErrAuth, nil
		}
	case lib.CredTypeSshKey:
		switch cr.(type) {
		case *CredentialsAsSSHKey:
			credentials := cr.(*CredentialsAsSSHKey)
			errCode, cred := lib.NewCredSshKey(credentials.UserName, credentials.PublicKeyPath, credentials.PrivateKeyPath, credentials.Passphrase)
			return lib.ErrorCode(errCode), &cred
		default:
			return lib.ErrAuth, nil
		}
	case lib.CredTypeSshCustom, lib.CredTypeDefault, 70:
		switch cr.(type) {
		case *CredentialsAsSSHAgent:
			credentials := cr.(*CredentialsAsSSHAgent)
			errCode, cred := lib.NewCredSshKeyFromAgent(credentials.UserName)
			return lib.ErrorCode(errCode), &cred
		default:
			return lib.ErrAuth, nil
		}
	default:
		return lib.ErrAuth, nil
	}
}

func defaultCertCheckCallback(opts OptionsWithCreds, cert *lib.Certificate, valid bool, hostname string) lib.ErrorCode {
	// TODO: look for certificate check
	return 0
}
