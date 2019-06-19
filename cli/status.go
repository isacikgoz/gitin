package cli

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/isacikgoz/gia/editor"
	"github.com/isacikgoz/gitin/prompt"
	"github.com/isacikgoz/gitin/term"
	git "github.com/isacikgoz/libgit2-api"
)

// status holds the repository struct and the prompt pointer.
type status struct {
	repository *git.Repository
	prompt     *prompt.Prompt
}

// StatusPrompt draws the screen with its list, initializing the cursor to the given position.
func StatusPrompt(r *git.Repository, opts *prompt.Options) (*prompt.Prompt, error) {
	st, err := r.LoadStatus()
	if err != nil {
		return nil, fmt.Errorf("could not load status: %v", err)
	}
	items := make([]prompt.Item, 0)
	for _, entry := range st.Entities {
		items = append(items, entry)
	}
	if len(items) == 0 {
		printClean(r)
		os.Exit(0)
	}
	list, err := prompt.NewList(items, opts.LineSize)
	if err != nil {
		return nil, fmt.Errorf("could not create list: %v", err)
	}
	controls := make(map[string]string)
	controls["add/reset entry"] = "space"
	controls["show diff"] = "enter"
	controls["add all"] = "a"
	controls["reset all"] = "r"
	controls["hunk stage"] = "p"
	controls["commit"] = "c"
	controls["amend"] = "m"
	controls["discard changes"] = "!"

	s := &status{repository: r}

	s.prompt = prompt.Create("Files", opts, list,
		prompt.WithKeyHandler(s.onKey),
		prompt.WithSelectionHandler(s.onSelect),
		prompt.WithItemRenderer(renderItem),
		prompt.WithInformation(s.branchInfo),
	)
	s.prompt.Controls = controls

	return s.prompt, nil
}

// return true to terminate
func (s *status) onSelect() error {
	item, err := s.prompt.Selection()
	if err != nil {
		return fmt.Errorf("can't show diff: %v", err)
	}
	entry := item.(*git.StatusEntry)
	if err = popGitCommand(s.repository, fileStatArgs(entry)); err != nil {
		// return fmt.Errorf("could not run a git command: %v", err)
		return nil // intentionally ignore errors here
	}
	return nil
}

func (s *status) onKey(key rune) error {
	var reqReload bool
	switch key {
	case ' ':
		reqReload = true
		s.addReset()
	case 'p':
		reqReload = true
		s.hunkStage()
	case 'c':
		reqReload = true
		s.doCommit()
	case 'm':
		reqReload = true
		s.doCommitAmend()
	case 'a':
		reqReload = true
		// TODO: check for errors
		addAll(s.repository)
	case 'r':
		reqReload = true
		resetAll(s.repository)
	case '!':
		reqReload = true
		s.discardChanges()
	case 'q':
		s.prompt.Stop()
	default:
	}
	if reqReload {
		if err := s.reloadStatus(); err != nil {
			return err
		}
	}
	return nil
}

// reloads the list
func (s *status) reloadStatus() error {
	s.repository.LoadHead()
	status, err := s.repository.LoadStatus()
	if err != nil {
		return err
	}
	items := make([]prompt.Item, 0)
	for _, entry := range status.Entities {
		items = append(items, entry)
	}
	if len(items) == 0 {
		// this is the case when the working tree is cleaned at runtime
		s.prompt.Stop()
		s.prompt.SetExitMsg(workingTreeClean(s.repository.Head))
		return nil
	}
	state := s.prompt.State()
	list, err := prompt.NewList(items, state.ListSize)
	if err != nil {
		return err
	}
	state.List = list
	s.prompt.SetState(state)
	return nil
}

// add or reset selected entry
func (s *status) addReset() error {
	item, err := s.prompt.Selection()
	if err != nil {
		return fmt.Errorf("can't add/reset item: %v", err)
	}
	entry := item.(*git.StatusEntry)
	args := []string{"add", "--", entry.String()}
	if entry.Indexed() {
		args = []string{"reset", "HEAD", "--", entry.String()}
	}
	cmd := exec.Command("git", args...)
	cmd.Dir = s.repository.Path()
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

// open hunk stagin ui
func (s *status) hunkStage() error {
	// defer s.prompt.writer.HideCursor()

	item, err := s.prompt.Selection()
	if err != nil {
		return fmt.Errorf("can't hunk stage item: %v", err)
	}
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
	} else {

	}
	return nil
}

func (s *status) doCommit() error {
	// defer s.prompt.writer.HideCursor()

	args := []string{"commit", "--edit", "--quiet"}
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
		return err
	}
	return nil
}

func (s *status) doCommitAmend() error {
	// defer s.prompt.writer.HideCursor()

	args := []string{"commit", "--amend", "--quiet"}
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
		return err
	}
	return nil
}

func (s *status) branchInfo(item prompt.Item) [][]term.Cell {
	b := s.repository.Head
	return branchInfo(b, true)
}

func (s *status) discardChanges() error {
	// defer s.prompt.render()
	item, err := s.prompt.Selection()
	if err != nil {
		return fmt.Errorf("cant't discard changes on item: %v", err)
	}
	entry := item.(*git.StatusEntry)
	args := []string{"checkout", "--", entry.String()}
	cmd := exec.Command("git", args...)
	cmd.Dir = s.repository.Path()
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

func printClean(r *git.Repository) {
	writer := term.NewBufferedWriter(os.Stdout)
	for _, line := range workingTreeClean(r.Head) {
		writer.WriteCells(line)
	}
	writer.Flush()
}
