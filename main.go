package main

import (
	"fmt"
	"os"

	"github.com/isacikgoz/sig/prompt"

	git "github.com/isacikgoz/libgit2-api"
	env "github.com/kelseyhightower/envconfig"
	pin "gopkg.in/alecthomas/kingpin.v2"
)

// Config will be passed to screenopts
type Config struct {
}

var (
	cfg    Config
	pwd, _ = os.Getwd()
	dir    = pin.Arg("directory", "If Sig is suppose to run elsewhere.").Default(pwd).String()
)

func main() {
	pin.Version("sig version 0.1.0")
	pin.UsageTemplate(pin.DefaultUsageTemplate + envVarHelp() + "\n")
	pin.CommandLine.HelpFlag.Short('h')
	pin.CommandLine.VersionFlag.Short('v')
	pin.Parse()
	mode := "status"
	if err := run(mode, *dir); err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
}

func run(mode, path string) error {
	r, err := git.Open(path)
	if err != nil {
		return err
	}
	opts := &prompt.Options{
		Size: 5,
	}
	switch mode {
	case "status":
		pr := prompt.Status{
			Repo: r,
		}

		s, err := r.LoadStatus()
		if err != nil {
			return err
		}
		items := make([]prompt.Item, 0)
		for _, entry := range s.Entities {
			items = append(items, entry)
		}
		pr.Items = items
		opts.SearchLabel = "Files"
		err = pr.Start(opts)
	case "log":
		pr := prompt.Log{
			Repo: r,
		}

		cs, err := r.Commits()
		if err != nil {
			return err
		}
		items := make([]prompt.Item, 0)
		for _, commit := range cs {
			items = append(items, commit)
		}
		pr.Items = items
		opts.SearchLabel = "Commits"
		err = pr.Start(opts)
	}
	return err
}

func configurePrompt(r *git.Repository) (*prompt.Status, error) {
	err := env.Process("sig", &cfg)
	if err != nil {
		return nil, err
	}

	prompt := prompt.Status{
		Repo: r,
	}
	return &prompt, nil
}

func envVarHelp() string {
	return `Environment Variables:
  None.`
}
