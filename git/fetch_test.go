package git

import (
	"os"
	"testing"
)

func TestFetch(t *testing.T) {
	repo, err := testCloneFromLocal("merge")
	defer os.RemoveAll(repo.path) // clean up
	checkFatal(t, err)

	creds := &CredentialsAsPlainText{}

	var tests = []struct {
		input  *FetchOptions
		output error
	}{
		{&FetchOptions{
			Remote:      "origin",
			Tags:        true,
			Credentials: creds,
		}, nil},
		{&FetchOptions{
			Remote:      "asda",
			Credentials: creds,
		}, ErrNotValidRemoteName},
		{&FetchOptions{
			Credentials: creds,
			Prune:       true,
		}, ErrNoRemoteName},
	}
	for _, test := range tests {
		if err := repo.Fetch(test.input); err != test.output {
			t.Errorf("input: %v error: %s\n", test.input, err.Error())
		}
	}
}
