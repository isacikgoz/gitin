package cli

import (
	"errors"
	"io"
	"os"
	"os/exec"

	log "github.com/sirupsen/logrus"
)

// PromptOptions is the common options for building a prompt
type PromptOptions struct {
	Cursor   int
	Scroll   int
	Size     int
	HideHelp bool
}

var (
	// NoErrRecurse is just a indactor to the caller to pop prompt back
	NoErrRecurse error = errors.New("catch")
)

func popMore(in string) error {
	os.Setenv("LESS", "-RCS")
	cmd := exec.Command("less")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		log.Fatal(err)
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	defer func() {
		cmd.Stdin = os.Stdin
	}()
	go func() {
		defer stdin.Close()
		io.WriteString(stdin, in)
	}()
	if err := cmd.Start(); err != nil {
		return err
	}
	if err := cmd.Wait(); err != nil {
		return err
	}
	return NoErrRecurse
}
