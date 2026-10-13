package git

import (
	"os"
	"os/exec"
	"path/filepath"
)

// xcodeSelectLink points to the developer directory chosen with
// xcode-select -s, without it Xcode or the command line tools are used
const xcodeSelectLink = "/var/db/xcode_select_link"

var defaultDeveloperDirs = []string{
	"/Applications/Xcode.app/Contents/Developer",
	"/Library/Developer/CommandLineTools",
}

// findGit returns the git to run. /usr/bin/git on macOS is a shim that looks
// up the developer tools and runs their git, which takes about as long as a
// short git command itself. gitin runs that git directly, and the shim if it
// can't tell which git the shim would run.
func findGit() string {
	return developerGit(exec.LookPath, os.Getenv("DEVELOPER_DIR"), xcodeSelectLink, defaultDeveloperDirs)
}

// developerGit returns the git of the developer directory the shim uses: the
// one DEVELOPER_DIR names, else the one chosen with xcode-select, else the
// first default one with a git.
func developerGit(lookPath func(string) (string, error), developerDir, selectLink string, defaultDirs []string) string {
	if path, err := lookPath("git"); err != nil || path != "/usr/bin/git" {
		return "git"
	}
	dirs := defaultDirs
	if developerDir != "" {
		// it can name Xcode.app instead of its developer directory
		dirs = []string{developerDir, filepath.Join(developerDir, "Contents", "Developer")}
	} else if link, err := os.Readlink(selectLink); err == nil {
		if !filepath.IsAbs(link) {
			link = filepath.Join(filepath.Dir(selectLink), link)
		}
		dirs = []string{link}
	}
	for _, dir := range dirs {
		path := filepath.Join(dir, "usr", "bin", "git")
		if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0o111 != 0 {
			return path
		}
	}
	return "git"
}
