package cli

import (
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/isacikgoz/gia/editor"
	"github.com/isacikgoz/gitin/prompt"
	"github.com/isacikgoz/gitin/term"
	git "github.com/isacikgoz/libgit2-api"
	"github.com/waigani/diffparser"
)

// status holds the repository struct and the prompt pointer.
type status struct {
	repository *git.Repository
	prompt     *prompt.Prompt
}

// StatusPrompt configures a prompt to serve as work-dir explorer prompt
func StatusPrompt(r *git.Repository, opts *prompt.Options) (*prompt.Prompt, error) {
	st, err := r.LoadStatus()
	if err != nil {
		return nil, fmt.Errorf("could not load status: %v", err)
	}
	if len(st.Entities) == 0 {
		writer := term.NewBufferedWriter(os.Stdout)
		for _, line := range workingTreeClean(r.Head) {
			writer.WriteCells(line)
		}
		writer.Flush()
		os.Exit(0)
	}
	list, err := prompt.NewList(st.Entities, opts.LineSize)
	if err != nil {
		return nil, fmt.Errorf("could not create list: %v", err)
	}

	s := &status{repository: r}

	s.prompt = prompt.Create("Files", opts, list,
		prompt.WithSelectionHandler(s.onSelect),
		prompt.WithItemRenderer(renderItem),
		prompt.WithInformation(s.info),
	)
	if err := s.defineKeybindings(); err != nil {
		return nil, err
	}

	return s.prompt, nil
}

// return err to terminate
func (s *status) onSelect(item interface{}) error {
	entry := item.(*git.StatusEntry)
	if err := popGitCommand(s.repository, fileStatArgs(entry)); err != nil {
		return nil // intentionally ignore errors here
	}
	return nil
}

func (s *status) info(item interface{}) [][]term.Cell {
	b := s.repository.Head
	return branchInfo(b, true)
}

func (s *status) defineKeybindings() error {
	keybindings := []*prompt.KeyBinding{
		&prompt.KeyBinding{
			Key:     ' ',
			Display: "space",
			Desc:    "add/reset entry",
			Handler: s.addResetEntry,
		},
		&prompt.KeyBinding{
			Key:     'p',
			Display: "p",
			Desc:    "hunk stage entry",
			Handler: s.hunkStageEntry,
		},
		&prompt.KeyBinding{
			Key:     'c',
			Display: "c",
			Desc:    "commit",
			Handler: s.commit,
		},
		&prompt.KeyBinding{
			Key:     'm',
			Display: "m",
			Desc:    "amend",
			Handler: s.amend,
		},
		&prompt.KeyBinding{
			Key:     'a',
			Display: "a",
			Desc:    "add all",
			Handler: s.addAllEntries,
		},
		&prompt.KeyBinding{
			Key:     'r',
			Display: "r",
			Desc:    "reset all",
			Handler: s.resetAllEntries,
		},
		&prompt.KeyBinding{
			Key:     '!',
			Display: "!",
			Desc:    "discard changes",
			Handler: s.checkoutEntry,
		},
		&prompt.KeyBinding{
			Key:     'q',
			Display: "q",
			Desc:    "quit",
			Handler: s.quit,
		},
	}
	for _, kb := range keybindings {
		if err := s.prompt.AddKeyBinding(kb); err != nil {
			return err
		}
	}
	return nil
}

func (s *status) addResetEntry(item interface{}) error {
	entry := item.(*git.StatusEntry)
	args := []string{"add", "--", entry.String()}
	if entry.Indexed() {
		args = []string{"reset", "HEAD", "--", entry.String()}
	}
	return s.runCommandWithArgs(args)
}

func (s *status) hunkStageEntry(item interface{}) error {
	entry := item.(*git.StatusEntry)
	file, err := generateDiffFile(s.repository, entry)
	if err == nil {
		editor, err := editor.NewEditor(file)
		if err != nil {
			return err
		}
		patches, err := editor.Run()
		if err != nil {
			return err
		}
		for _, patch := range patches {
			if err := applyPatchCmd(s.repository, entry, patch); err != nil {
				return err
			}
		}
	}
	return s.reloadStatus()
}

func (s *status) commit(item interface{}) error {
	return s.bareCommit("--edit")
}

func (s *status) amend(item interface{}) error {
	return s.bareCommit("--amend")
}

func (s *status) bareCommit(arg string) error {
	args := []string{"commit", arg, "--quiet"}
	err := popGitCommand(s.repository, args)
	if err != nil {
		return err
	}
	s.repository.LoadHead()
	args, err = lastCommitArgs(s.repository)
	if err != nil {
		return err
	}
	if err := popGitCommand(s.repository, args); err != nil {
		return fmt.Errorf("failed to commit: %v", err)
	}
	return s.reloadStatus()
}

func (s *status) addAllEntries(item interface{}) error {
	args := []string{"add", "."}
	return s.runCommandWithArgs(args)
}

func (s *status) resetAllEntries(item interface{}) error {
	args := []string{"reset", "--mixed"}
	return s.runCommandWithArgs(args)
}

func (s *status) checkoutEntry(item interface{}) error {
	entry := item.(*git.StatusEntry)
	args := []string{"checkout", "--", entry.String()}
	return s.runCommandWithArgs(args)
}

func (s *status) quit(item interface{}) error {
	s.prompt.Stop()
	return nil
}

func (s *status) runCommandWithArgs(args []string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = s.repository.Path()
	if err := cmd.Run(); err != nil {
		return nil //ignore command errors for now
	}
	return s.reloadStatus()
}

// reloads the list
func (s *status) reloadStatus() error {
	s.repository.LoadHead()
	status, err := s.repository.LoadStatus()
	if err != nil {
		return err
	}
	if len(status.Entities) == 0 {
		// this is the case when the working tree is cleaned at runtime
		s.prompt.Stop()
		s.prompt.SetExitMsg(workingTreeClean(s.repository.Head))
		return nil
	}
	state := s.prompt.State()
	list, err := prompt.NewList(status.Entities, state.ListSize)
	if err != nil {
		return err
	}
	state.List = list
	s.prompt.SetState(state)
	return nil
}

// fileStatArgs returns git command args for getting diff
func fileStatArgs(e *git.StatusEntry) []string {
	var args []string
	if e.Indexed() {
		args = []string{"diff", "--cached", e.String()}
	} else if e.EntryType == git.StatusEntryTypeUntracked {
		args = []string{"diff", "--no-index", "/dev/null", e.String()}
	} else {
		args = []string{"diff", "--", e.String()}
	}
	return args
}

// lastCommitArgs returns the args for show stat
func lastCommitArgs(r *git.Repository) ([]string, error) {
	r.LoadStatus()
	head := r.Head
	if head == nil {
		return nil, fmt.Errorf("can't get HEAD")
	}
	hash := string(head.Target().Hash)
	args := []string{"show", "--stat", hash}
	return args, nil
}

func generateDiffFile(r *git.Repository, entry *git.StatusEntry) (*diffparser.DiffFile, error) {
	args := fileStatArgs(entry)
	cmd := exec.Command("git", args...)
	cmd.Dir = r.Path()
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, err
	}
	diff, err := diffparser.Parse(string(out))
	if err != nil {
		return nil, err
	}
	return diff.Files[0], nil
}

func applyPatchCmd(r *git.Repository, entry *git.StatusEntry, patch string) error {
	mode := []string{"apply", "--cached"}
	if entry.Indexed() {
		mode = []string{"apply", "--cached", "--reverse"}
	}
	cmd := exec.Command("git", mode...)
	cmd.Dir = r.Path()
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	go func() {
		defer stdin.Close()
		io.WriteString(stdin, patch+"\n")
	}()
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}
