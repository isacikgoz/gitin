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

	if err := run(*dir); err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
}

func run(path string) error {
	r, err := git.Open(path)
	if err != nil {
		return err
	}

	status, err := configurePrompt(r)
	if err != nil {
		return err
	}

	s, err := r.LoadStatus()
	if err != nil {
		return err
	}
	items := make([]prompt.Item, 0)
	for _, entry := range s.Entities {
		items = append(items, entry)
	}
	status.Items = items
	opts := &prompt.Options{
		Cursor:      0,
		Scroll:      0,
		Size:        5,
		SearchLabel: "Files: ",
	}
	err = status.Start(opts)
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
