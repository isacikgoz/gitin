package main

import (
	"fmt"
	"os"

	"github.com/isacikgoz/gitin/cli"
	"github.com/isacikgoz/gitin/prompt"

	git "github.com/isacikgoz/libgit2-api"
	env "github.com/kelseyhightower/envconfig"
	pin "gopkg.in/alecthomas/kingpin.v2"
)

func main() {
	mode := evalArgs()
	pwd, _ := os.Getwd()

	r, err := git.Open(pwd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	var opts prompt.Options
	err = env.Process("gitin", &opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	switch mode {
	case "status":
		err = cli.StatusPrompt(r, &opts)
	case "log":
		err = cli.LogPrompt(r, &opts)
	case "branch":
		err = cli.BranchPrompt(r, &opts)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

// define the program commands and args
func evalArgs() string {
	pin.Command("log", "Show commit logs.")
	pin.Command("status", "Show working-tree status. Also stage and commit changes.")
	pin.Command("branch", "Show list of branches.")

	pin.Version("gitin version 0.2.1")

	pin.UsageTemplate(pin.DefaultUsageTemplate + additionalHelp() + "\n")
	pin.CommandLine.HelpFlag.Short('h')
	pin.CommandLine.VersionFlag.Short('v')

	return pin.Parse()
}

func additionalHelp() string {
	return `Environment Variables:

  GITIN_LINESIZE=<int>
  GITIN_STARTINSEARCH=<bool>
  GITIN_DISABLECOLOR=<bool>

Press ? for controls while application is running.`
}
