package git

import (
	"os"
	"testing"
)

func TestMerge(t *testing.T) {
	repo, err := testCloneFromLocal("merge")
	defer os.RemoveAll(repo.path) // clean up
	checkFatal(t, err)

	var tests = []struct {
		branch string
		input  *MergeOptions
		output error
	}{
		{"asd", &MergeOptions{
			NoFF: true,
		}, ErrBranchNotFound},
		{"origin/master", &MergeOptions{
			IgnoreAlreadyUpToDate: true,
		}, nil},
	}
	for _, test := range tests {
		if err := repo.Merge(test.branch, test.input); err != test.output {
			t.Errorf("input branch: %s, error: %s\n", test.branch, err.Error())
		}
	}
}
