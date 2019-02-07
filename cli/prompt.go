package cli

import (
	"errors"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/isacikgoz/gitin/git"
	"github.com/isacikgoz/promptui"
	"github.com/micmonay/keybd_event"
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

func popGitCmd(r *git.Repository, args []string) error {
	os.Setenv("LESS", "-RCS")
	cmd := exec.Command("git", args...)
	cmd.Dir = r.AbsPath

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Start(); err != nil {
		log.Warn(err.Error())
	}
	if err := cmd.Wait(); err != nil {
		log.Warn(err.Error())
	}
	return NoErrRecurse
}

func currentOptions(prompt *promptui.Select, opts *PromptOptions) *PromptOptions {
	return &PromptOptions{
		Cursor:   prompt.CursorPosition(),
		Scroll:   prompt.ScrollPosition(),
		Size:     opts.Size,
		HideHelp: opts.HideHelp,
	}
}

func emuEnterKey() error {
	if runtime.GOOS == "linux" {
		return errors.New("not supported on linux")
	}
	kb, err := keybd_event.NewKeyBonding()
	if err != nil {
		return err
	}

	//set keys
	kb.SetKeys(keybd_event.VK_ENTER)
	err = kb.Launching()
	if err != nil {
		return err
	}
	return nil
}

func quitPrompt(r *git.Repository, chb chan bool) {
	r.Close()
	chb <- true
	// sorry to steal 100 ms, lets give it to readline to close itself
	time.Sleep(100 * time.Millisecond)
	os.Exit(0)
}
