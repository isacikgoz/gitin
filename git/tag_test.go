package git

import (
	"os"
	"testing"
)

func TestTags(t *testing.T) {
	repo, err := testCloneFromLocal("commit")
	defer os.RemoveAll(repo.path) // clean up
	checkFatal(t, err)

	_, err = repo.Tags()
	if err != nil {
		t.Errorf("got error: %s\n", err.Error())
	}
}
