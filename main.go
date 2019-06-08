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
	cfg       Config
	logCmd    = pin.Command("log", "Show commit logs.")
	statusCmd = pin.Command("status", "Show working-tree status. Also stage and commit changes.")
	branchCmd = pin.Command("branch", "Show list of branches.")
)

func main() {
	pin.Version("sig version 0.1.0")
	pin.UsageTemplate(pin.DefaultUsageTemplate + envVarHelp() + "\n")
	pin.CommandLine.HelpFlag.Short('h')
	pin.CommandLine.VersionFlag.Short('v')
	pin.Parse()
	pwd, _ := os.Getwd()
	if err := run(pwd); err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
}

func run(path string) error {
	r, err := git.Open(path)
	if err != nil {
		return err
	}
	opts := &prompt.Options{
		Size: 5,
	}
	switch pin.Parse() {
	case "status":
		pr := prompt.Status{
			Repo: r,
		}
		err = pr.Start(opts)
	case "log":
		pr := prompt.Log{
			Repo: r,
		}
		err = pr.Start(opts)
	case "branch":
		pr := prompt.Branch{
			Repo: r,
		}
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
