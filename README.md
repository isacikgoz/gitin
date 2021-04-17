![](https://img.shields.io/circleci/build/github/isacikgoz/gitin/master) ![](https://img.shields.io/github/release-pre/isacikgoz/gitin.svg?style=flat)

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
- Convenient UX and minimalist design
- See more options by running `gitin --help`, also you can get help for individual subcommands (e.g. `gitin log --help`)

## Installation

- Linux and macOS are supported, Windows is not at the moment.
- Download latest release from [here](https://github.com/isacikgoz/gitin/releases)
- **Or**, manually download it with `go get -d github.com/isacikgoz/gitin/cmd/gitin`
- `cd` into `$GOPATH/src/github.com/isacikgoz/gitin`
- build with `make install` (`cmake` and `pkg-config` are required, also note that git2go will be cloned and built)

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
  -h, --help     Show context-sensitive help (also try --help-long and --help-man).
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

Environment Variables:

  GITIN_LINESIZE=<int>
  GITIN_STARTINSEARCH=<bool
  GITIN_DISABLECOLOR=<bool>
  GITIN_VIMKEYS=<bool>

Press ? for controls while application is running.

```

## Configure

- To set the line size `export GITIN_LINESIZE=5`
- To set always start in search mode `GITIN_STARTINSEARCH=true`
- To disable colors `GITIN_DISABLECOLOR=true`
- To disable h,j,k,l for nav `GITIN_VIMKEYS=false`

## Development Requirements

- **Running with static linking is highly recommended.**
- Clone the project and `cd` into it.
- Run `make build-libgit2` (this will satisfy the replace rule in the `go.mod` file)
- You can run the project with `go run --tags static cmd/gitin/main.go --help` command

## Contribution

- Contributions are welcome. If you like to please refer to [Contribution Guidelines](/CONTRIBUTING.md)
- Bug reports should include descriptive steps to reproduce so that maintainers can easily understand the actual problem
- Feature requests are welcome, ask for anything that seems appropriate

## Credits

See the [credits page](https://github.com/isacikgoz/gitin/wiki/Credits)

## License

[BSD-3-Clause](/LICENSE)
