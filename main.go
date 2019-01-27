package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/isacikgoz/gitin/cli"
	"github.com/isacikgoz/gitin/git"

	log "github.com/sirupsen/logrus"
)

func main() {
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
	if len(os.Args) < 2 {
		return errors.New("usage: gitin <command>\n\nCommands:\n  log\n  status")
	}
	if os.Args[1] == "log" {
		return cli.Log(r, 0)
	} else if os.Args[1] == "status" {
		return cli.Status(r, 0)
	}
	return nil
}
