//go:build !darwin

package git

// findGit returns the git to run
func findGit() string {
	return "git"
}
