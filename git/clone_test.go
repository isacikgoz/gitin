package git

import (
	"io/ioutil"
	"os"
	"testing"
)

func TestClone(t *testing.T) {
	dirs := make([]string, 0)
	wd, _ := os.Getwd()
	for i := 0; i < 2; i++ {
		dir, err := ioutil.TempDir("", "temp-clone-dir")
		checkFatal(t, err)
		defer os.RemoveAll(dir) // clean up
		dirs = append(dirs, dir)
	}
	creds := &CredentialsAsPlainText{}
	opts := &CloneOptions{
		Credentials: creds,
	}
	var tests = []struct {
		inputDir string
		inputURL string
		inputOpt *CloneOptions
		err      error
	}{
		{dirs[0], wd, opts, nil},
	}
	for _, test := range tests {
		if _, err := Clone(test.inputDir, test.inputURL, test.inputOpt); err != test.err {
			t.Errorf("Test Failed. dir: %s, url: %s inputted, got \"%s\" as error.",
				test.inputDir, test.inputURL, err.Error())
		}
	}
}
