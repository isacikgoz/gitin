package git

import (
	"os"
	"testing"
)

func TestBranches(t *testing.T) {
	repo, err := testCloneFromLocal("commit")
	checkFatal(t, err)
	defer os.RemoveAll(repo.path) // clean up

	_, err = repo.Branches()
	if err != nil {
		t.Errorf("got error: %s\n", err.Error())
	}
}
