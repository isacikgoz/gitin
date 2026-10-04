package main

import (
	"context"
	"fmt"
	"os"
	"runtime/debug"

	"github.com/isacikgoz/gitin/cli"
	"github.com/isacikgoz/gitin/git"
	"github.com/isacikgoz/gitin/prompt"

	env "github.com/kelseyhightower/envconfig"
	pin "gopkg.in/alecthomas/kingpin.v2"
)

// version is set at build time with -ldflags "-X main.version=..."
var version = ""

// arguments of the hunk editor gitin runs itself
var hunkDiff, hunkPatches *string

func main() {
	mode := evalArgs()
	if mode == cli.HunkEditorCommand {
		exitIfError(cli.RunHunkEditor(*hunkDiff, *hunkPatches))
		return
	}
	pwd, _ := os.Getwd()

	r, err := git.Open(pwd)
	exitIfError(err)

	var o prompt.Options
	err = env.Process("gitin", &o)
	exitIfError(err)

	var p *prompt.Prompt
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// cli package is for responsible to create and configure a prompt
	switch mode {
	case "status":
		p, err = cli.StatusPrompt(r, &o)
	case "log":
		p, err = cli.LogPrompt(ctx, r, &o)
	case "branch":
		p, err = cli.BranchPrompt(r, &o)
	case "push":
		exitIfError(cli.Push(ctx, r, &o))
		return
	default:
		return
	}

	exitIfError(err)
	if p == nil {
		return // nothing to show, e.g. the working tree is clean
	}
	if err := p.Run(ctx); err != nil {
		cancel()
		exitIfError(err)
	}
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
	pin.Command("push", "Push the current branch, optionally after running the checks of .gitin.yml.")
	hunks := pin.Command(cli.HunkEditorCommand, "Pick hunks of a diff, used by the status command.").Hidden()
	hunkDiff = hunks.Arg("diff", "File with the diff of a file.").Required().String()
	hunkPatches = hunks.Arg("patches", "File to write the patches of the picked hunks to.").Required().String()

	pin.Version("gitin version " + buildVersion())

	pin.UsageTemplate(pin.DefaultUsageTemplate + additionalHelp() + "\n")
	pin.CommandLine.HelpFlag.Short('h')
	pin.CommandLine.VersionFlag.Short('v')

	return pin.Parse()
}

func buildVersion() string {
	if version != "" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return "dev"
}

func additionalHelp() string {
	return `Environment Variables:

  GITIN_LINESIZE=<int>
  GITIN_STARTINSEARCH=<bool>
  GITIN_DISABLECOLOR=<bool>
  GITIN_VIMKEYS=<bool>

Press ? for controls while application is running.`
}
