package git

import (
	"os"
	"testing"

	lib "github.com/libgit2/git2go"
)

func TestDefaultAuthCallbackFunc(t *testing.T) {
	wd, _ := os.Getwd()

	var tests = []struct {
		inputOpts     OptionsWithCreds
		inputURL      string
		inoutUsr      string
		inputCred     lib.CredType
		outErrCode    lib.ErrorCode
		outCredential *lib.Cred
	}{
		{&CloneOptions{
			Credentials: &CredentialsAsPlainText{},
		}, wd, "git", lib.CredTypeUserpassPlaintext, lib.ErrOk, nil},
		{&CloneOptions{
			Credentials: &CredentialsAsSSHKey{},
		}, "git@github.com:isacikgoz/gia.git", "git", lib.CredTypeSshKey, lib.ErrOk, nil},
		{&CloneOptions{
			Credentials: &CredentialsAsSSHKey{},
		}, "git@github.com:isacikgoz/gia.git", "git", lib.CredTypeUserpassPlaintext, lib.ErrAuth, nil},
		{&CloneOptions{
			Credentials: &CredentialsAsSSHAgent{},
		}, "ssh://github.com/git/git", "git", lib.CredTypeUserpassPlaintext, lib.ErrAuth, nil},
	}
	for _, test := range tests {
		if errCode, _ := defaultAuthCallback(test.inputOpts, test.inputURL, test.inoutUsr, test.inputCred); errCode != test.outErrCode {
			t.Errorf("test failed: for input url: %s, got error code: %d\n", test.inputURL, errCode)
		}
	}
}

func TestDefaultCertCheckCallback(t *testing.T) {
	opts := &CloneOptions{}
	var tests = []struct {
		inputOpts  OptionsWithCreds
		inputCert  *lib.Certificate
		inputValid bool
		inputHost  string
		outErrCode lib.ErrorCode
	}{
		{opts, nil, false, "", 0},
	}
	for _, test := range tests {
		if errCode := defaultCertCheckCallback(test.inputOpts, test.inputCert, test.inputValid, test.inputHost); errCode != test.outErrCode {
			t.Error("test failed.")
		}
	}
}
