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
	LineSize    int `default:"5"`
	StartSearch bool
}

var (
	cfg       Config
	logCmd    = pin.Command("log", "Show commit logs.")
	statusCmd = pin.Command("status", "Show working-tree status. Also stage and commit changes.")
	branchCmd = pin.Command("branch", "Show list of branches.")
)

func main() {
	evalArgs()

	pwd, _ := os.Getwd()
	if err := run(pwd); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func evalArgs() {
	pin.Version("gitin version 0.2.0")
	pin.UsageTemplate(pin.DefaultUsageTemplate + envVarHelp() + "\n")
	pin.CommandLine.HelpFlag.Short('h')
	pin.CommandLine.VersionFlag.Short('v')
}

func run(path string) error {
	r, err := git.Open(path)
	if err != nil {
		return err
	}
	err = env.Process("gitin", &cfg)
	if err != nil {
		return err
	}
	opts := &prompt.Options{
		Size:          cfg.LineSize,
		StartInSearch: cfg.StartSearch,
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

func envVarHelp() string {
	return `Environment Variables:

  GITIN_LINESIZE=<int>
  GITIN_STARTSEARCH=<bool>

  Press ? for controls while application is running.`
}
