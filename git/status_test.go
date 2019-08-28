package git

import (
	"io/ioutil"
	"os"
	"testing"
)

func TestAddToIndex(t *testing.T) {
	repo, err := testCloneFromLocal("add")
	checkFatal(t, err)

	defer os.RemoveAll(repo.path) // clean up
	status, err := repo.LoadStatus()
	checkFatal(t, err)

	// create a file to add
	err = writeTestFile(repo.path + "/added.go")
	checkFatal(t, err)

	// get the status entries
	status, err = repo.LoadStatus()
	checkFatal(t, err)

	var tests = []struct {
		input  *StatusEntry
		output error
	}{
		{status.Entities[0], nil},
	}
	for _, test := range tests {
		if err := repo.AddToIndex(test.input); err != test.output {
			t.Errorf("input: %s, output: %s\n", test.input.diffDelta.OldFile.Path, err.Error())
		}
	}
}

func TestRemoveFromIndex(t *testing.T) {
	repo, err := testCloneFromLocal("reset")
	defer os.RemoveAll(repo.path) // clean up
	checkFatal(t, err)

	// create a file to add
	err = writeTestFile(repo.path + "/added.go")
	checkFatal(t, err)

	// create this file to see that it is not included into commit
	err = writeTestFile(repo.path + "/untracked.go")
	checkFatal(t, err)

	// get the status entries
	status, err := repo.LoadStatus()
	checkFatal(t, err)

	err = repo.AddToIndex(status.Entities[0])
	checkFatal(t, err)

	// reload status to get new file stats
	status, err = repo.LoadStatus()
	checkFatal(t, err)

	var tests = []struct {
		input  *StatusEntry
		output error
	}{
		{status.Entities[0], nil},
		{status.Entities[1], ErrEntryNotIndexed},
	}
	for _, test := range tests {
		if err := repo.RemoveFromIndex(test.input); err != test.output {
			t.Errorf("input: %s, output: %s\n", test.input.diffDelta.OldFile.Path, err.Error())
		}
	}
}

func writeTestFile(path string) error {
	d1 := []byte("package git\n\nimport \"fmt\"\n\nfunc test() {\n\tfmt.Println(\"a\")\n}\n")
	return ioutil.WriteFile(path, d1, 0644)
}
