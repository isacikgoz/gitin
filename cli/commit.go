package cli

import (
	"errors"

	"github.com/isacikgoz/gitin/git"
	"github.com/isacikgoz/promptui"
)

type CommitOptions struct {
	MinCommitLength int
	MaxCommitLength int
	Message         string
	PromptOps       *PromptOptions
}

func CommitBuilder(r *git.Repository, opts *CommitOptions) error {
	if len(opts.Message) <= 0 {
		opts.Message = "message"
	}
	return commitPrompt(r, opts)
}
func commitPrompt(r *git.Repository, opts *CommitOptions) error {
	validate := func(input string) error {
		if len(input) < 3 {
			return errors.New("commits must have more than 3 characters")
		}
		return nil
	}

	prompt := promptui.Prompt{
		Label:    "git commit -m",
		Validate: validate,
		Default:  opts.Message,
	}

	text, err := prompt.Run()

	if err != nil {
		return err
	}
	out, err := r.DoCommit(text)
	if err != nil {
		return err
	}
	if err := r.InitializeStatus(); err != nil {
		return err
	}
	if err := popMore(out); err != nil {
		return err
	}
	return NoErrRecurse
}

func commitTemplate(r *git.Repository) *promptui.SelectTemplates {
	templates := &promptui.SelectTemplates{
		Label:    "{{ . |yellow}}:",
		Active:   "* {{- if .Indexed }} {{ printf \"%.1s\" .StatusEntryString | green}}{{- else}} {{ printf \"%.1s\" .StatusEntryString | red}}{{- end}} {{ .String }}",
		Inactive: "  {{- if .Indexed }}  {{ printf \"%.1s\" .StatusEntryString | green}}{{- else}}  {{ printf \"%.1s\" .StatusEntryString | red}}{{- end}} {{ .String }}",
		Selected: "{{ .String }}",
		Extra:    "add/reset: space commit: m",
		Details: "\n" +
			"---------------- Status -----------------" + "\n" +
			"{{ \"On branch\" }} " + "{{ \"" + r.Branch.Name + "\" | yellow }}" + "\n" +
			getAheadBehind(r.Branch),
	}
	return templates
}
