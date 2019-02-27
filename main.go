package main

import (
	"fmt"
	"os"
	"sync"

	"github.com/isacikgoz/fig/git"
	"github.com/isacikgoz/sig/prompt"

	env "github.com/kelseyhightower/envconfig"
	log "github.com/sirupsen/logrus"
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

	log.SetLevel(log.ErrorLevel)

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

	screen, err := configureScreen(r)
	if err != nil {
		return err
	}
	var mx sync.Mutex
	done := make(chan bool)

	adder := func(incoming []git.FuzzItem) {
		mx.Lock()
		defer mx.Unlock()
		screen.AppendToMainList(incoming)
	}

	go r.LoadStatusEntries(adder, done)

	go func() {
		if <-done {
			log.Debug("loading finished")
		}
	}()
	err = screen.Start(0, 0)
	return err
}

func configureScreen(r *git.Repository) (*prompt.Status, error) {
	err := env.Process("sig", &cfg)
	if err != nil {
		return nil, err
	}

	screen := prompt.Status{
		Repo: r,
	}
	return &screen, nil
}

func envVarHelp() string {
	return `Environment Variables:
  None.`
}
