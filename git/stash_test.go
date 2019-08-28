package git

import (
	"os"
	"testing"
	"time"
)

func TestStashes(t *testing.T) {
	r, err := testCloneFromLocal("stash")
	checkFatal(t, err)

	defer os.RemoveAll(r.path) // clean up

	// create a file to add
	err = writeTestFile(r.path + "/added.go")
	checkFatal(t, err)

	// create this file to see that it is not included into stash
	err = writeTestFile(r.path + "/untracked.go")
	checkFatal(t, err)

	// get the status entries
	status, err := r.LoadStatus()
	checkFatal(t, err)

	err = r.AddToIndex(status.Entities[0])
	checkFatal(t, err)

	sig := &Signature{
		Name:  "Some Guy",
		Email: "guysome@gmail.com",
		When:  time.Now(),
	}
	_, err = r.AddToStash(sig, "a stashed item", StashDefault)
	if err != nil {
		t.Errorf("test failed: %v", err)
	}
}
