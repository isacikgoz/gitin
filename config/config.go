// Package config reads the configuration of gitin. A repository can have
// two configuration files: a personal one in the git directory, which is
// never committed, and an optional shared one in the working tree, which is
// committed for the team.
package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"go.yaml.in/yaml/v3"
)

// SharedFileNames are the names of the shared configuration file in the
// root of the working tree, in the order they are looked up
var SharedFileNames = []string{".gitin.yml", ".gitin.yaml"}

// PersonalFile is the path of the personal configuration file in the git
// directory
var PersonalFile = filepath.Join("gitin", "config.yml")

// Config is the content of the configuration files
type Config struct {
	// Files are the files the configuration was read from, as the user
	// refers to them from the root of the working tree
	Files []string `yaml:"-"`
	// Personal is the personal configuration file, also if it doesn't exist
	Personal string `yaml:"-"`
	// Commit runs before committing in "gitin status"
	Commit Hook `yaml:"commit"`
	// Push runs before "gitin push" pushes
	Push Hook `yaml:"push"`
}

// Hook lists the checks that run before an operation
type Hook struct {
	// Checks run in order, the operation stops at the first failure
	Checks []Check `yaml:"checks"`
}

// Check is a command that has to succeed before the operation
type Check struct {
	// Name is shown to the user, it defaults to Run
	Name string `yaml:"name"`
	// Run is a shell command, it runs in the root of the working tree
	Run string `yaml:"run"`
}

// Load reads the configuration of a repository: the shared file in the
// root of its working tree, if it has one, and the personal file in its
// common git directory, shared by all its worktrees. The checks of both
// run, the shared ones first. Missing files are no error.
func Load(worktree, commonDir string) (*Config, error) {
	c := &Config{}
	var sources []string
	if worktree != "" {
		var found []string
		for _, name := range SharedFileNames {
			if _, err := os.Stat(filepath.Join(worktree, name)); err == nil {
				found = append(found, filepath.Join(worktree, name))
			}
		}
		if len(found) > 1 {
			return nil, fmt.Errorf("both %s and %s exist, keep one of them", SharedFileNames[0], SharedFileNames[1])
		}
		sources = append(sources, found...)
	}
	personal := filepath.Join(commonDir, PersonalFile)
	c.Personal = display(worktree, personal)
	if _, err := os.Stat(personal); err == nil {
		sources = append(sources, personal)
	}

	for _, path := range sources {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		name := display(worktree, path)
		file, err := parse(data)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		c.Files = append(c.Files, name)
		c.Commit.Checks = append(c.Commit.Checks, file.Commit.Checks...)
		c.Push.Checks = append(c.Push.Checks, file.Push.Checks...)
	}
	return c, nil
}

// display returns path relative to the working tree if it is inside it
func display(worktree, path string) string {
	if worktree == "" {
		return path
	}
	rel, err := filepath.Rel(worktree, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return path
	}
	return rel
}

func parse(data []byte) (*Config, error) {
	c := &Config{}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	// typos must not silently disable checks
	dec.KnownFields(true)
	if err := dec.Decode(c); err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	hooks := []struct {
		name string
		hook *Hook
	}{{"commit", &c.Commit}, {"push", &c.Push}}
	for _, h := range hooks {
		name, hook := h.name, h.hook
		for i := range hook.Checks {
			check := &hook.Checks[i]
			if check.Run == "" {
				return nil, fmt.Errorf("%s.checks[%d]: run is missing, it is the command of the check", name, i)
			}
			if check.Name == "" {
				check.Name = check.Run
			}
		}
	}
	return c, nil
}
