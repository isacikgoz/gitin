package main

import (
	"fmt"
	"os"

	"github.com/isacikgoz/gitin/cli"
	"github.com/isacikgoz/gitin/git"

	env "github.com/kelseyhightower/envconfig"
	log "github.com/sirupsen/logrus"
	pin "gopkg.in/alecthomas/kingpin.v2"
)

type Config struct {
	LineSize int
	HideHelp bool
}

var (
	cfg             Config
	branchCommand   = pin.Command("branch", "Checkout, list, or delete branches.")
	branchAll       = branchCommand.Flag("all", "list both remote and local branches").Bool()
	branchRemotes   = branchCommand.Flag("remote", "list only remote branches").Bool()
	branchOrderDate = branchCommand.Flag("date-order", "order branches by date").Bool()
	logCommand      = pin.Command("log", "Show commit logs.")
	logAhead        = logCommand.Flag("ahead", "show commits that not pushed to upstream").Bool()
	logAuthor       = logCommand.Flag("author", "limit commits to those by given author").String()
	logBefore       = logCommand.Flag("before", "show commits older than given date (RFC3339)").String()
	logBehind       = logCommand.Flag("behind", "show commits that not merged from upstream").Bool()
	logCommitter    = logCommand.Flag("committer", "limit commits to those by given committer").String()
	logMaxCount     = logCommand.Flag("max-count", "maximum number of commits to display").Int()
	logTags         = logCommand.Flag("tags", "show tags alongside commits").Bool()
	logSince        = logCommand.Flag("since", "show commits newer than given date (RFC3339)").String()
	status          = pin.Command("status", "Show working-tree status. Also stage and commit changes.")
)

func main() {

	pin.Version("gitin version 0.1.4")
	pin.CommandLine.HelpFlag.Short('h')
	pin.CommandLine.VersionFlag.Short('v')
	pin.Parse()
	err := env.Process("gitin", &cfg)
	if err != nil {
		log.Fatal(err.Error())
	}
	log.SetLevel(log.ErrorLevel)
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
	promptOps := &cli.PromptOptions{
		Cursor:   0,
		Scroll:   0,
		Size:     cfg.LineSize,
		HideHelp: cfg.HideHelp,
	}
	switch pin.Parse() {
	case "branch":
		orderType := cli.BranchSortDefault
		if *branchOrderDate {
			orderType = cli.BranchSortDate
		}
		types := cli.LocalBranches
		if *branchAll {
			types = cli.AllBranches
		} else if *branchRemotes {
			types = cli.RemoteBranches
		}
		opts := &cli.BranchOptions{
			Types:     types,
			PromptOps: promptOps,
			Sort:      orderType,
		}
		return cli.BranchBuilder(r, opts)
	case "log":
		opts := &cli.LogOptions{
			Author:    *logAuthor,
			Before:    *logBefore,
			Committer: *logCommitter,
			Tags:      *logTags,
			MaxCount:  *logMaxCount,
			Since:     *logSince,
			PromptOps: promptOps,
		}
		if *logAhead {
			opts.Mode = cli.LogAhead
			return cli.LogBuilder(r, opts)
		} else if *logBehind {
			opts.Mode = cli.LogBehind
			return cli.LogBuilder(r, opts)
		} else {
			opts.Mode = cli.LogNormal
			return cli.LogBuilder(r, opts)
		}
	case "status":
		opts := &cli.StatusOptions{
			PromptOps: promptOps,
		}
		return cli.StatusBuilder(r, opts)
	}
	return nil
}
