package main

import (
	"fmt"
	"context"
	"os"

	"github.com/isacikgoz/gitin/cli"
	"github.com/isacikgoz/gitin/git"
	"github.com/isacikgoz/gitin/prompt"

	env "github.com/kelseyhightower/envconfig"
	pin "gopkg.in/alecthomas/kingpin.v2"
)

func main() {
	mode := evalArgs()
	pwd, _ := os.Getwd()

	r, err := git.Open(pwd)
	exitIfError(err)

	var o prompt.Options
	err = env.Process("gitin", &o)
	exitIfError(err)

	var p *prompt.Prompt

	// cli package is for responsible to create and configure a prompt
	switch mode {
	case "status":
		p, err = cli.StatusPrompt(r, &o)
	case "log":
		p, err = cli.LogPrompt(r, &o)
	case "branch":
		p, err = cli.BranchPrompt(r, &o)
	default:
		return
	}

	exitIfError(err)
	ctx := context.Background()
	exitIfError(p.Run(ctx))
}

func exitIfError(err error) {
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

	pin.Version("gitin version 0.2.3")

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
