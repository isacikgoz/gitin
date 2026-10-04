package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/isacikgoz/gitin/git"
	"github.com/isacikgoz/gitin/prompt"
	"github.com/isacikgoz/gitin/term"
	"github.com/waigani/diffparser"
)

// status holds the repository struct and the prompt pointer.
type status struct {
	repository *git.Repository
	prompt     *prompt.Prompt
	branch     *git.Branch
}

// StatusPrompt configures a prompt to serve as work-dir explorer prompt. If
// the working tree is clean, it prints that and returns no prompt.
func StatusPrompt(r *git.Repository, opts *prompt.Options) (*prompt.Prompt, error) {
	s, err := newStatus(r, opts)
	if err != nil || s == nil {
		return nil, err
	}
	return s.prompt, nil
}

func newStatus(r *git.Repository, opts *prompt.Options) (*status, error) {
	st, err := r.LoadStatus()
	if err != nil {
		return nil, fmt.Errorf("could not load status: %v", err)
	}
	if len(st.Entities) == 0 {
		writer := term.NewBufferedWriter(stdout)
		for _, line := range workingTreeClean(st.Branch) {
			if _, err := writer.WriteCells(line); err != nil {
				return nil, err
			}
		}
		return nil, writer.Flush()
	}
	list, err := prompt.NewList(st.Entities, opts.LineSize)
	if err != nil {
		return nil, fmt.Errorf("could not create list: %v", err)
	}

	s := &status{repository: r, branch: st.Branch}

	s.prompt = prompt.Create("Files", opts, list,
		prompt.WithSelectionHandler(s.onSelect),
		prompt.WithItemRenderer(renderItem),
		prompt.WithInformation(s.info),
	)
	if err := s.defineKeybindings(); err != nil {
		return nil, err
	}

	return s, nil
}

func (s *status) onSelect(item interface{}) error {
	entry := item.(*git.StatusEntry)
	err := popGitCommand(s.repository, fileStatArgs(entry))
	if entry.EntryType == git.StatusEntryTypeUntracked && git.ExitCode(err) == 1 {
		return nil // "git diff --no-index" exits with 1 when the files differ
	}
	return err
}

func (s *status) info(item interface{}) [][]term.Cell {
	return branchInfo(s.branch, true)
}

func (s *status) defineKeybindings() error {
	keybindings := []*prompt.KeyBinding{
		{
			Key:     ' ',
			Display: "space",
			Desc:    "add/reset entry",
			Handler: s.addResetEntry,
		},
		{
			Key:     'p',
			Display: "p",
			Desc:    "hunk stage entry",
			Handler: s.hunkStageEntry,
		},
		{
			Key:     'c',
			Display: "c",
			Desc:    "commit",
			Handler: s.commit,
		},
		{
			Key:     'm',
			Display: "m",
			Desc:    "amend",
			Handler: s.amend,
		},
		{
			Key:     'a',
			Display: "a",
			Desc:    "add all",
			Handler: s.addAllEntries,
		},
		{
			Key:     'r',
			Display: "r",
			Desc:    "reset all",
			Handler: s.resetAllEntries,
		},
		{
			Key:     '!',
			Display: "!",
			Desc:    "discard changes",
			Handler: s.discardEntry,
		},
		{
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
		// unlike "reset HEAD", this also works before the first commit
		args = []string{"reset", "--quiet", "--", entry.String()}
	}
	return s.runCommandWithArgs(args)
}

func (s *status) hunkStageEntry(item interface{}) error {
	entry := item.(*git.StatusEntry)
	diff, err := entryDiff(s.repository, entry)
	if err != nil {
		return err
	}
	if _, err := parseDiffFile(diff, entry.String()); err != nil {
		return err
	}
	patches, err := runHunkEditor(diff)
	if err != nil {
		return err
	}
	for _, patch := range patches {
		if err := applyPatchCmd(s.repository, entry, patch); err != nil {
			return err
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
	if err := popGitCommand(s.repository, []string{"commit", arg, "--quiet"}); err != nil {
		return err // e.g. a hook rejected the commit or the message was empty
	}
	if err := popGitCommand(s.repository, []string{"show", "--stat", "HEAD"}); err != nil {
		return err
	}
	return s.reloadStatus()
}

func (s *status) addAllEntries(item interface{}) error {
	return s.runCommandWithArgs([]string{"add", "--all"})
}

func (s *status) resetAllEntries(item interface{}) error {
	return s.runCommandWithArgs([]string{"reset", "--quiet", "--mixed"})
}

func (s *status) discardEntry(item interface{}) error {
	entry := item.(*git.StatusEntry)
	var args []string
	switch {
	case entry.Indexed():
		return errors.New("staged changes are not discarded, press space to unstage them first")
	case entry.EntryType == git.StatusEntryTypeUntracked:
		// -d removes untracked directories with older git versions as well
		args = []string{"clean", "-d", "--force", "--", entry.String()}
	default:
		args = []string{"checkout", "--", entry.String()}
	}
	return s.runCommandWithArgs(args)
}

func (s *status) quit(item interface{}) error {
	s.prompt.Stop()
	return nil
}

func (s *status) runCommandWithArgs(args []string) error {
	if err := runGitCommand(s.repository, args); err != nil {
		return err
	}
	return s.reloadStatus()
}

// reloads the list
func (s *status) reloadStatus() error {
	status, err := s.repository.LoadStatus()
	if err != nil {
		return err
	}
	s.branch = status.Branch
	if len(status.Entities) == 0 {
		// this is the case when the working tree is cleaned at runtime
		s.prompt.Stop()
		s.prompt.SetExitMsg(workingTreeClean(s.branch))
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
	switch {
	case e.Indexed():
		return []string{"diff", "--cached", "--", e.String()}
	case e.EntryType == git.StatusEntryTypeUntracked:
		return []string{"diff", "--no-index", "--", "/dev/null", e.String()}
	default:
		return []string{"diff", "--", e.String()}
	}
}

func generateDiffFile(r *git.Repository, entry *git.StatusEntry) (*diffparser.DiffFile, error) {
	diff, err := entryDiff(r, entry)
	if err != nil {
		return nil, err
	}
	return parseDiffFile(diff, entry.String())
}

// entryDiff returns the diff of an entry as a patch for "git apply",
// whatever the user's diff settings are
func entryDiff(r *git.Repository, entry *git.StatusEntry) ([]byte, error) {
	args := fileStatArgs(entry)
	args = append([]string{args[0], "--no-color", "--no-ext-diff", "--no-textconv", "--src-prefix=a/", "--dst-prefix=b/"}, args[1:]...)
	out, err := r.Output(args...)
	if entry.EntryType == git.StatusEntryTypeUntracked && git.ExitCode(err) == 1 {
		err = nil // "git diff --no-index" exits with 1 when the files differ
	}
	return out, err
}

// parseDiffFile parses the diff of a single file with at least one hunk
func parseDiffFile(out []byte, name string) (*diffparser.DiffFile, error) {
	diff, err := diffparser.Parse(string(out))
	if err != nil {
		return nil, err
	}
	// e.g. binary files and mode changes have no hunks
	if len(diff.Files) == 0 || len(diff.Files[0].Hunks) == 0 {
		return nil, fmt.Errorf("%s has no changes to stage by hunk", name)
	}
	file := diff.Files[0]
	// diffparser drops header lines such as "new file mode" and the file
	// names after them, "git apply" needs the header exactly as git wrote it
	if i := strings.Index(string(out), "\n@@"); i >= 0 {
		file.DiffHeader = string(out[:i])
	}
	return file, nil
}

func applyPatchCmd(r *git.Repository, entry *git.StatusEntry, patch string) error {
	args := []string{"apply", "--cached"}
	if entry.Indexed() {
		args = append(args, "--reverse")
	}
	cmd := r.Command(args...)
	cmd.Stdin = strings.NewReader(patch + "\n")
	_, err := git.Run(cmd)
	return err
}
