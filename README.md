# gitin

`gitin` is a true interactive cli to `git`

![screenshot](https://files.catbox.moe/w8l2ld.gif)

## Installation Requirements
- Requires gitlib2 v27 and `git2go`. See the project homepages for build instructions.
  1. download git2go; `go get -d gopkg.in/libgit2/git2go.v27`
  2. make syre you have `cmake` `libssl-dev` installed
  3. `cd` into `$GOPATH/src/gopkg.in/libgit2/git2go.v27`
  4. initialize submodules by running `git submodule update --init`
  5. change the libigt2 version to your version (in this case its 0.27) in the install script (`script/install-libgit2.sh`)
  6. run the script `./script/install-libgit2.sh`
- After these you can install with `go get github.com/isacikgoz/gitin`

## Disclaimer
This project is at very early stage of the development and only `gitin log` is implemented to see the concept.
