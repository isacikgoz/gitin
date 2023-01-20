package git

import (
	"os"
	"runtime"
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

// got form got2go tests, seems useful
func checkFatal(t *testing.T, err error) {
	if err == nil {
		return
	}

	// The failure happens at wherever we were called, not here
	_, file, line, ok := runtime.Caller(1)
	if !ok {
		t.Fatalf("Unable to get caller")
	}
	t.Fatalf("Fail at %v:%v; %v", file, line, err)
}
