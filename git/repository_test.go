package git

import (
	"os"
	"testing"
)

func TestOpen(t *testing.T) {
	wd, _ := os.Getwd()
	var tests = []struct {
		input string
		err   error
	}{
		{"/tmp", ErrCannotOpenRepo},
		{"/", ErrCannotOpenRepo},
		{wd, nil},
	}
	for _, test := range tests {
		if _, err := Open(test.input); err != test.err {
			t.Errorf("input: %s\n error: %s", test.input, err.Error())
		}
	}
}
