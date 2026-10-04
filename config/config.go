// Package config reads the configuration of gitin from the root of the
// working tree, it is meant to be committed and shared with the team.
package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"go.yaml.in/yaml/v3"
)

// FileNames are the names of the configuration file, in the order they are looked up
var FileNames = []string{".gitin.yml", ".gitin.yaml"}

// Config is the content of the configuration file
type Config struct {
	// File is the name of the file the configuration was read from, empty
	// if there is none
	File string `yaml:"-"`
	Push Push   `yaml:"push"`
}

// Push configures "gitin push"
type Push struct {
	// Checks run before pushing, in order, the push stops at the first failure
	Checks []Check `yaml:"checks"`
}

// Check is a command that has to succeed before pushing
type Check struct {
	// Name is shown to the user, it defaults to Run
	Name string `yaml:"name"`
	// Run is a shell command, it runs in the root of the working tree
	Run string `yaml:"run"`
}

// Load reads the configuration file in dir. Without one it returns an empty configuration.
func Load(dir string) (*Config, error) {
	var found []string
	for _, name := range FileNames {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			found = append(found, name)
		}
	}
	switch len(found) {
	case 0:
		return &Config{}, nil
	case 1:
	default:
		return nil, fmt.Errorf("both %s and %s exist, keep one of them", found[0], found[1])
	}

	data, err := os.ReadFile(filepath.Join(dir, found[0]))
	if err != nil {
		return nil, err
	}
	c, err := parse(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", found[0], err)
	}
	c.File = found[0]
	return c, nil
}

func parse(data []byte) (*Config, error) {
	c := &Config{}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	// typos must not silently disable checks
	dec.KnownFields(true)
	if err := dec.Decode(c); err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	for i := range c.Push.Checks {
		check := &c.Push.Checks[i]
		if check.Run == "" {
			return nil, fmt.Errorf("push.checks[%d]: run is missing, it is the command of the check", i)
		}
		if check.Name == "" {
			check.Name = check.Run
		}
	}
	return c, nil
}
