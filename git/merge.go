package git

import (
	lib "gopkg.in/libgit2/git2go.v27"
)

// MergeOptions defines common options for merge operation
type MergeOptions struct {
	Message               string
	NoFF                  bool
	FailOnConflict        bool
	IgnoreAlreadyUpToDate bool
}

// Merge incorporates changes from the given branch into the current branch
func (r *Repository) Merge(ref string, opts *MergeOptions) error {
	repo := r.essence
	theirhead, err := repo.LookupBranch(ref, lib.BranchRemote)
	if err != nil {
		return ErrBranchNotFound
	}
	heads := make([]*lib.AnnotatedCommit, 0)
	ano, err := repo.AnnotatedCommitFromRef(theirhead.Reference)
	if err != nil {
		return err
	}
	heads = append(heads, ano)
	analysis, _, err := repo.MergeAnalysis(heads)
	if err != nil {
		return err
	}
	switch analysis {
	case lib.MergeAnalysisUpToDate:
		if !opts.IgnoreAlreadyUpToDate {
			return ErrAlreadyUpToDate
		}
	case lib.MergeAnalysisFastForward:
		if opts.NoFF {
			return ErrFastForwardOnly
		}
	}
	options, err := lib.DefaultMergeOptions()
	if err != nil {
		return err
	}
	checkoutOptions := &lib.CheckoutOpts{}
	err = repo.Merge(heads, &options, checkoutOptions)
	if err != nil {
		return err
	}
	return nil
}
