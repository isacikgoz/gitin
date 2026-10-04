[![CI](https://github.com/isacikgoz/gitin/actions/workflows/ci.yml/badge.svg?branch=master)](https://github.com/isacikgoz/gitin/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/isacikgoz/gitin?include_prereleases&sort=semver)](https://github.com/isacikgoz/gitin/releases/latest)
[![Go Reference](https://pkg.go.dev/badge/github.com/isacikgoz/gitin.svg)](https://pkg.go.dev/github.com/isacikgoz/gitin)
[![Go version](https://img.shields.io/github/go-mod/go-version/isacikgoz/gitin)](go.mod)
[![License](https://img.shields.io/github/license/isacikgoz/gitin)](LICENSE)

# gitin

`gitin` is a commit/branch/status explorer for `git`

gitin is a minimalist tool that lets you explore a git repository from the command line. You can search from commits, inspect individual files and changes in the commits. It is an alternative and interactive way to explore the commit history. Also, you can explore your current state by investigating diffs, stage your changes and commit them.

<p align="center">
   <img src="https://user-images.githubusercontent.com/2153367/59564874-98fed180-9054-11e9-9341-1b2801268194.gif" alt="screencast"/>
</p>

## Features

- Fuzzy search (type `/` to start a search after running `gitin <command>`)
- Interactive stage and see the diff of files (`gitin status` then press `enter` to see diff or `space` to stage)
- Commit/amend changes (`gitin status` then press `c` to commit or `m` to amend)
- Interactive hunk staging (`gitin status` then press `p`)
- Explore branches with useful filter options (e.g. `gitin branch` press `enter` to checkout)
- Push with checks: `gitin push` runs the checks of [`.gitin.yml`](#checks-before-pushing) before pushing, or skips them if you choose to
- Fast on large repositories, the history is loaded and searched in the background
- Convenient UX and minimalist design
- See more options by running `gitin --help`, also you can get help for individual subcommands (e.g. `gitin log --help`)

## Installation

Linux and macOS are supported. gitin runs `git` to read repositories, `git` 2.18 or newer is required.

- Download the binary for your platform from the [latest release](https://github.com/isacikgoz/gitin/releases/latest).
  To check that it was built by this repository's release workflow, run `gh attestation verify <archive> --repo isacikgoz/gitin`
- **Or**, install it with Go: `go install github.com/isacikgoz/gitin/cmd/gitin@latest`

### Mac/Linux using brew

The tap is recently moved to new repo, so if you added the older one (isacikgoz/gitin), consider removing it and adding the new one.

```sh
brew tap isacikgoz/taps
brew install gitin
```

## Usage

```sh
usage: gitin [<flags>] <command> [<args> ...]

Flags:
  -h, --help     Show context-sensitive help (also try --help-long and
                 --help-man).
  -v, --version  Show application version.

Commands:
  help [<command>...]
    Show help.

  log
    Show commit logs.

  status
    Show working-tree status. Also stage and commit changes.

  branch
    Show list of branches.

  push
    Push the current branch, optionally after running the checks of .gitin.yml.


Environment Variables:

  GITIN_LINESIZE=<int>
  GITIN_STARTINSEARCH=<bool>
  GITIN_DISABLECOLOR=<bool>
  GITIN_VIMKEYS=<bool>

Press ? for controls while application is running.
```

## Configure

- To set the line size `export GITIN_LINESIZE=5`
- To set always start in search mode `GITIN_STARTINSEARCH=true`
- To disable colors `GITIN_DISABLECOLOR=true`
- To disable h,j,k,l for nav `GITIN_VIMKEYS=false`

### Checks before pushing

`gitin push` shows what it pushes and asks whether to run the checks of `.gitin.yml` first or to push without them, `q` cancels.
Commit the file to share the checks with your team:

```yaml
push:
  checks:
    - name: Lint
      run: golangci-lint run
    - name: Tests
      run: go test ./...
    - name: Build
      run: |
        make build
        ./scripts/smoke-test.sh
```

- Checks run one after the other in the root of the working tree, their output is shown as it comes.
- If a check fails, gitin asks whether to push anyway. Ctrl-C stops the running check and cancels the push.
- `run` is a shell command. Commands with `: ` in them have to be quoted, a `|` block is the easiest way.
- A branch without upstream is pushed to `origin` (or `remote.pushDefault`) and tracks it. git's own `pre-push` hook still runs.

## Development

Go and `git` are the only requirements.

```sh
go run ./cmd/gitin --help   # run gitin
make test                   # unit and end-to-end tests
make coverage               # the same with a coverage report
make dist VERSION=v1.2.3    # release archives for Linux and macOS
```

The end-to-end tests in [e2e](e2e) run the gitin binary in a pseudo terminal, type keys and check the screen and the repository.
[CI](.github/workflows/ci.yml) runs them on Linux and macOS and with the oldest supported Go and git versions, and checks lint, workflows and known vulnerabilities.
Pushing a `v*.*.*` tag runs CI and drafts a [release](.github/workflows/release.yml) with binaries for Linux and macOS and their build attestations, publish it on GitHub after reviewing the notes.

## Contribution

- Contributions are welcome. If you like to please refer to [Contribution Guidelines](CONTRIBUTING.md)
- Bug reports should include descriptive steps to reproduce so that maintainers can easily understand the actual problem
- Feature requests are welcome, ask for anything that seems appropriate

## Credits

See the [credits page](https://github.com/isacikgoz/gitin/wiki/Credits)

## License

[BSD-3-Clause](LICENSE)
