# gitin

`gitin` is a commit/branch/status explorer for `git`

gitin is a minimalist tool that lets you explore a git repository from command line. You can search from commits, inspect indvidual files and changes in the commits, see ahead/behind commits etc. It is an alternative and interactive way to explore the commit history. Also, you can explore your current state by investigating diffs, stage your changes and commit them.

<p align="center">
   <img src="https://github.com/isacikgoz/gitin/blob/master/img/screencast.gif" alt="screencast"/>
</p>

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
    Show working-tree status. Also stage and commit changes.

```

## Configure
- To set line size `export GITIN_LINESIZE=5`
- To hide help `export GITIN_HIDEHELP=true`

## Development Requirements
- Requires gitlib2 v27 and `git2go`. See the project homepages for build instructions.
  1. download git2go; `go get -d gopkg.in/libgit2/git2go.v27`
  2. make sure you have `cmake`, `pkg-confid` and `libssl-dev` installed
  3. `cd` into `$GOPATH/src/gopkg.in/libgit2/git2go.v27`
  4. initialize submodules by running `git submodule update --init`
  5. change the libigt2 version to your version (in this case its 0.27) in the install script (`script/install-libgit2.sh`)
  6. run the script `./script/install-libgit2.sh`
- After these you can download it with `go get github.com/isacikgoz/gitin`
- `cd` into `$GOPATH/src/github.com/isacikgoz/gitin` and start hacking

## Disclaimer
This project is at very early stage of the development and there may be a few bugs. Consider reporting them by raising an issue.

## License
[MIT](/LICENSE)
