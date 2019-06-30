![](https://img.shields.io/travis/com/isacikgoz/gitin.svg?style=flat) ![](https://img.shields.io/github/downloads/isacikgoz/gitin/total.svg?style=flat) ![](https://img.shields.io/github/release-pre/isacikgoz/gitin.svg?style=flat)

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
- **Or**, manually download it with `go get -d github.com/isacikgoz/gitin`
- `cd` into `$GOPATH/src/github.com/isacikgoz/gitin`
- make would expect a built libgit2 library to make a static link. So, when you run `make` command, you should be able to build libgit2 at your `$GOPATH/pkg/mod/gopkg.in/libgit2/git2go.../vendor/libgit2/build` directory. This issue has been shown up after go modules.
- build with `make install` (`cmake` and `pkg-config` are required)

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

- Requires gitlib2 v27 and `git2go`. See the project homepages for more information about build instructions. For gitin you can simply;
  - macOS:
    1. install libgit2 via `brew install libgit2` (consider that libgit2.v27 is required)
  - Linux and macOS(if you want to build your own):
    1. download git2go; `go get -d gopkg.in/libgit2/git2go.v27`
    2. make sure you have `cmake`, `pkg-config` and `libssl-dev` installed
    3. `cd` into `$GOPATH/src/gopkg.in/libgit2/git2go.v27`
    4. initialize submodules by running `git submodule update --init`
    5. change the libigt2 version to your version (in this case its 0.27) in the install script (e.g. `nano script/install-libgit2.sh` or `vim script/install-libgit2.sh`) and change `LG2VER` to 0.27.0
    6. run the script `./script/install-libgit2.sh`
- After these you can download it with `go get github.com/isacikgoz/gitin`
- `cd` into `$GOPATH/src/github.com/isacikgoz/gitin` and start hacking

## Contribution

- Contributions are welcome. If you like to please refer to [Contribution Guidelines](/CONTRIBUTING.md)
- Bug reports should include descriptive steps to reproduce so that maintainers can easily understand the actual problem
- Feature requests are welcome, ask for anything that seems appropriate

## Credits

See the [credits page](https://github.com/isacikgoz/gitin/wiki/Credits)

## License

[BSD-3-Clause](/LICENSE)
