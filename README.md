# gitin

`gitin` is a commit/branch/status explorer for `git`

gitin is a minimalist tool that lets you explore a git repository from the command line. You can search from commits, inspect individual files and changes in the commits. It is an alternative and interactive way to explore the commit history. Also, you can explore your current state by investigating diffs, stage your changes and commit them.

<p align="center">
   <img src="https://github.com/isacikgoz/gitin/blob/master/img/screencast.gif" alt="screencast"/>
</p>

## Features
- Various filters for commit history (e.g. `gitin log --author="..."`)
- Interactive stage and see the diff of files (`gitin status` then press `space`)
- Explore branches with useful filter options (`gitin branch` press `enter` to checkout)
- Commit/amend changes (`gitin status` then press `c` to commit or `m` to amend)
- See ahead/behind commits (e.g. `gitin log --ahead`)
- Interactive hunk staging (`gitin status` then press `p`)
- Minimalist design. Does not use the whole screen of your terminal
- See more options by simply `gitin --help`, also you can get help for individual subcommands (e.g. `gitin log --help`)

## Installation
- Works on Linux and macOS
- Download latest release from [here](https://github.com/isacikgoz/gitin/releases)
- Or, manually download it with `go get -d github.com/isacikgoz/gitin`
- `cd` into `$GOPATH/src/github.com/isacikgoz/gitin`
- build with `make install` (`cmake` and `pkg-config` are required)

### Mac using brew
```
brew tap isacikgoz/gitin
brew install gitin
```

## Usage
```bash
usage: gitin [<flags>] <command> [<args> ...]

Flags:
  -h, --help     Show context-sensitive help (also try --help-long and --help-man).
  -v, --version  Show application version.

Commands:
  help [<command>...]
    Show help.

  branch [<flags>]
    Checkout, list, or delete branches.

  log [<flags>]
    Show commit logs.

  status
    Show working-tree status. Also, stage and commit changes.

```

## Configure
- To set the line size `export GITIN_LINESIZE=5`
- To hide help `export GITIN_HIDEHELP=true`

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
- See the [credits page](https://github.com/isacikgoz/gitin/wiki/Credits)

## License
[BSD-3-Clause](/LICENSE)
