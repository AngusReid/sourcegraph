package local

import (
	"bytes"
	"fmt"
	"runtime"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	"github.com/rogpeppe/rog-go/parallel"
	"golang.org/x/net/context"
	"sourcegraph.com/sourcegraph/go-diff/diff"
	"src.sourcegraph.com/sourcegraph/errcode"
	"src.sourcegraph.com/sourcegraph/go-sourcegraph/sourcegraph"
	"src.sourcegraph.com/sourcegraph/pkg/vcs"
	"src.sourcegraph.com/sourcegraph/sourcecode"
	"src.sourcegraph.com/sourcegraph/svc"
)

// Maximum accepted raw diff size, in bytes. This is not the actual
// size of the returned structure, but only of the raw diff. The
// size of the actual payload may end up to be up to exponentially
// larger when tokenizing and linking head, base and raw diff source.
const defaultMaxDiffSize = 350 * 1024 // 350 KB

func (s *deltas) ListFiles(ctx context.Context, op *sourcegraph.DeltasListFilesOp) (*sourcegraph.DeltaFiles, error) {
	ds := op.Ds
	opt := op.Opt

	// Make sure we've fully resolved the RepoRevSpecs. If we haven't,
	// then they will need to be re-resolved in each call to
	// RepoTree.Get that we issue, which will seriously degrade
	// performance.
	resolveAndCacheRepoRevAndBranchExistence := func(ctx context.Context, repoRev *sourcegraph.RepoRevSpec) (context.Context, error) {
		// Cache repo so that our repeated calls to RepoTree.Get do
		// not need to repeatedly call RepoVCS.Open.
		vcsRepo, err := cachedRepoVCSOpen(ctx, repoRev.URI)
		if err != nil {
			return nil, err
		}
		ctx = withCachedRepo(ctx, repoRev.URI, vcsRepo)

		if repoRev.Resolved() {
			// The repo rev appears resolved already -- but it might have been
			// deleted, thus making any URLs we would emit for Rev instead of CommitID
			// invalid. Check if the rev/branch was deleted:
			//
			// TODO(slimsag): write a test exactly for this case.
			unresolvedRev := *repoRev
			unresolvedRev.CommitID = ""
			if err := (&repos{}).resolveRepoRev(ctx, &unresolvedRev); errcode.GRPC(err) == codes.NotFound {
				// Rev no longer exists, so fallback to the CommitID instead. This is a
				// last-ditch effort to ensure tokenized source displays well in diffs
				// that are very old / have had one or more of their revs/branches
				// deleted.
				repoRev.Rev = repoRev.CommitID
			} else if err != nil {
				return nil, err
			}
		}
		return ctx, (&repos{}).resolveRepoRev(ctx, repoRev)
	}
	ctx, err := resolveAndCacheRepoRevAndBranchExistence(ctx, &ds.Base)
	if err != nil {
		return nil, err
	}
	ctx, err = resolveAndCacheRepoRevAndBranchExistence(ctx, &ds.Head)
	if err != nil {
		return nil, err
	}

	if opt == nil {
		opt = &sourcegraph.DeltaListFilesOptions{}
	}
	if opt.MaxSize == 0 {
		opt.MaxSize = defaultMaxDiffSize
	}

	fdiffsAll, delta, err := s.diff(ctx, ds)
	if err != nil {
		return nil, err
	}

	var fdiffs []*diff.FileDiff
	if opt.Filter != "" {
		filter := opt.Filter
		expected := true
		if filter[0] == '!' {
			filter = filter[1:]
			expected = false
		}
		for _, fdiff := range fdiffsAll {
			if (strings.HasPrefix(fdiff.OrigName, filter) || strings.HasPrefix(fdiff.NewName, filter)) == expected {
				fdiffs = append(fdiffs, fdiff)
			}
		}
	} else {
		fdiffs = fdiffsAll
	}

	files, err := parseMultiFileDiffs(ctx, delta, fdiffs, opt)
	if err != nil {
		return nil, err
	}

	if opt.Formatted {
		// Parse and code-format file diffs.
		if err := formatFileDiffs(ctx, ds, files.FileDiffs); err != nil {
			return nil, err
		}
	}

	return files, nil
}

func (s *deltas) diff(ctx context.Context, ds sourcegraph.DeltaSpec) ([]*diff.FileDiff, *sourcegraph.Delta, error) {
	if s.mockDiffFunc != nil {
		return s.mockDiffFunc(ctx, ds)
	}

	delta, err := s.Get(ctx, &ds)
	if err != nil {
		return nil, nil, err
	}
	ds = delta.DeltaSpec()

	baseVCSRepo, err := cachedRepoVCSOpen(ctx, delta.BaseRepo.URI)
	if err != nil {
		return nil, nil, err
	}

	var headVCSRepo vcs.Repository
	sameRepo := ds.Base.RepoSpec != ds.Head.RepoSpec
	if sameRepo {
		headVCSRepo = baseVCSRepo
	} else {
		var err error
		headVCSRepo, err = cachedRepoVCSOpen(ctx, delta.HeadRepo.URI)
		if err != nil {
			return nil, nil, err
		}
	}

	var vcsDiff *vcs.Diff
	diffOpt := &vcs.DiffOptions{
		DetectRenames: true,
		OrigPrefix:    "",
		NewPrefix:     "",

		// We want `git diff base...head` not `git diff base..head` or
		// else branches with base merge commits show diffs that
		// include those merges, which isn't what we want (since those
		// merge commits are already reflected in the base).
		ExcludeReachableFromBoth: true,
	}

	if sameRepo {
		baseDiffer, ok := baseVCSRepo.(vcs.Differ)
		if !ok {
			return nil, nil, grpc.Errorf(codes.Unimplemented, fmt.Sprintf("repository %T does not support diffs", baseVCSRepo))
		}
		vcsDiff, err = baseDiffer.Diff(vcs.CommitID(ds.Base.CommitID), vcs.CommitID(ds.Head.CommitID), diffOpt)
		if err != nil {
			return nil, nil, err
		}
	} else {
		baseDiffer, ok := baseVCSRepo.(vcs.CrossRepoDiffer)
		if !ok {
			return nil, nil, grpc.Errorf(codes.Unimplemented, fmt.Sprintf("repository %T does not support cross-repo diffs", baseVCSRepo))
		}
		vcsDiff, err = baseDiffer.CrossRepoDiff(vcs.CommitID(ds.Base.CommitID), headVCSRepo, vcs.CommitID(ds.Head.CommitID), diffOpt)
		if err != nil {
			return nil, nil, err
		}
	}

	fdiffs, err := diff.ParseMultiFileDiff([]byte(vcsDiff.Raw))
	if err != nil {
		return nil, nil, err
	}
	return fdiffs, delta, nil
}

// parseMultiFileDiffs converts a slice of diff.FileDiffs to a slice of sourcegraph.FileDiff,
// applying syntax-highlighting and adding various information.
func parseMultiFileDiffs(ctx context.Context, delta *sourcegraph.Delta, fdiffs []*diff.FileDiff, opt *sourcegraph.DeltaListFilesOptions) (*sourcegraph.DeltaFiles, error) {
	var overSized bool
	if opt.MaxSize > 0 && len(fdiffs) > 1 {
		var totalSize int
		for _, fd := range fdiffs {
			for _, h := range fd.Hunks {
				totalSize += len(h.Body)
			}
		}
		if int32(totalSize) > opt.MaxSize {
			overSized = true
		}
	}
	par := parallel.NewRun(runtime.GOMAXPROCS(0))
	fds := make([]*sourcegraph.FileDiff, len(fdiffs))
	for i, fd := range fdiffs {
		parseRenames(fd)
		pre, post := getPrePostImage(fd.Extended)
		fds[i] = &sourcegraph.FileDiff{
			FileDiff:      *fd,
			FileDiffHunks: make([]*sourcegraph.Hunk, len(fd.Hunks)),
			Stats:         fd.Stat(),
			PreImage:      pre,
			PostImage:     post,
		}
		for j, h := range fd.Hunks {
			hunk := &sourcegraph.Hunk{Hunk: *h}
			hunkFileDiff := fds[i]
			fds[i].FileDiffHunks[j] = hunk
			if opt.Tokenized && !overSized {
				par.Do(func() error {
					tokenizeHunkBody(hunkFileDiff, hunk)
					linkBaseAndHead(ctx, delta, hunkFileDiff, hunk)
					return nil
				})
			}
		}
	}
	if err := par.Wait(); err != nil {
		return nil, err
	}
	files := &sourcegraph.DeltaFiles{
		FileDiffs:     fds,
		Delta:         delta,
		OverThreshold: overSized,
	}
	files.Stats = files.DiffStat()
	return files, nil
}

// parseRenames checks if this file diff is barely a rename and updates
// it's OrigName and NewName values accordingly from extended headers
// "rename from <path>" and "rename to <path>" if available.
// This only occurs on renames with similarity index at 100% which contain
// no hunks.
func parseRenames(fd *diff.FileDiff) {
	if fd.Hunks != nil || fd.OrigName != "" {
		// this is not a rename
		return
	}
	var prefixFrom = "rename from "
	var prefixTo = "rename to "
	for _, h := range fd.Extended {
		if strings.HasPrefix(h, prefixFrom) {
			fd.OrigName = h[len(prefixFrom):]
			continue
		}
		if strings.HasPrefix(h, prefixTo) {
			fd.NewName = h[len(prefixTo):]
			break
		}
	}
}

// getPrePostImage searches for a diff's index header inside a list
// of headers and if found, returns the pre and post commit ID or
// empty strings.
func getPrePostImage(headers []string) (pre, post string) {
	for _, h := range headers {
		if strings.HasPrefix(h, "index") {
			n, err := fmt.Sscanf(h, "index %40s..%40s", &pre, &post)
			if n == 2 && err == nil {
				if pre == strings.Repeat("0", 40) {
					pre = ""
				}
				if post == strings.Repeat("0", 40) {
					post = ""
				}
				return
			}
			break
		}
	}
	return "", ""
}

// tokenizeHunkBody removes diff prefixes such as '+', '-', ' ' from the body of
// the hunk and stores the clean body, as well as the tokenized one.
func tokenizeHunkBody(fd *sourcegraph.FileDiff, hunk *sourcegraph.Hunk) {
	var prefixes, body bytes.Buffer
	for _, l := range strings.Split(string(hunk.Body), "\n") {
		if len(l) > 0 {
			prefixes.WriteByte(l[0])
		}
		if len(l) > 1 {
			body.WriteString(l[1:])
		}
		body.WriteString("\n")
	}
	hunk.LinePrefixes = prefixes.String()

	file := sourcegraph.FileWithRange{
		BasicTreeEntry: &sourcegraph.BasicTreeEntry{Contents: body.Bytes()},
	}
	fileName := fd.NewName
	if fd.NewName == "/dev/null" {
		fileName = fd.OrigName
	}
	file.Name = fileName

	hunk.Body = nil
	hunk.BodySource = sourcecode.Tokenize(&file)
	// compute word-diff
	wordDiff(hunk)
}

// fetchCodeSnippet fetches a snippet of code from the VCS, applying syntax highlighting
// and linking to it.
func fetchCodeSnippet(ctx context.Context, spec sourcegraph.TreeEntrySpec, fileRange sourcegraph.FileRange) *sourcegraph.SourceCode {
	opt := sourcegraph.RepoTreeGetOptions{
		TokenizedSource: true,
		GetFileOptions:  sourcegraph.GetFileOptions{FileRange: fileRange},
	}
	entry, err := svc.RepoTree(ctx).Get(ctx, &sourcegraph.RepoTreeGetOp{Entry: spec, Opt: &opt})
	// If any errors occur while fetching the snippet, resume execution and don't block
	// the user experience. Content will still be available as a fall back from the
	// BodySource entry, but might not be linked on this hunk.
	// This will occur very rarely, such as for example on git submodule entries, where
	// the tree might not be available.
	if err == nil {
		return entry.SourceCode
	}
	return nil
}

// linkBaseAndHead applies syntax highlight and linking to both revisions in a hunk,
// if they are considered to be code and have successful builds.
func linkBaseAndHead(ctx context.Context, delta *sourcegraph.Delta, fd *sourcegraph.FileDiff, hunk *sourcegraph.Hunk) {
	if fd.OrigName != "/dev/null" {
		fileRange := sourcegraph.FileRange{
			StartLine: int64(hunk.OrigStartLine),
			EndLine:   int64(hunk.OrigStartLine + hunk.OrigLines - 1),
		}
		spec := sourcegraph.TreeEntrySpec{RepoRev: delta.Base, Path: fd.OrigName}
		baseLines := strings.Count(hunk.LinePrefixes, " ") + strings.Count(hunk.LinePrefixes, "-")
		if base := fetchCodeSnippet(ctx, spec, fileRange); base != nil && len(base.Lines) >= baseLines {
			var bl int
			for i, p := range hunk.LinePrefixes {
				switch p {
				case '-':
					hunk.BodySource.Lines[i] = base.Lines[bl]
					bl++
				case ' ':
					bl++
				}
			}
		}
	}
	if fd.NewName != "/dev/null" {
		fileRange := sourcegraph.FileRange{
			StartLine: int64(hunk.NewStartLine),
			EndLine:   int64(hunk.NewStartLine + hunk.NewLines - 1),
		}
		spec := sourcegraph.TreeEntrySpec{RepoRev: delta.Head, Path: fd.NewName}
		headLines := strings.Count(hunk.LinePrefixes, " ") + strings.Count(hunk.LinePrefixes, "+")
		if head := fetchCodeSnippet(ctx, spec, fileRange); head != nil && len(head.Lines) >= headLines {
			var hl int
			for i, p := range hunk.LinePrefixes {
				switch p {
				case '+', ' ':
					hunk.BodySource.Lines[i] = head.Lines[hl]
					hl++
				}
			}
		}
	}
}

// formatFileDiffs applies code formatting (syntax highlighting and
// reference linking) to all diff hunk bodies. It modifies the hunk
// bodies.
func formatFileDiffs(ctx context.Context, ds sourcegraph.DeltaSpec, diffs []*sourcegraph.FileDiff) error {
	par := parallel.NewRun(runtime.GOMAXPROCS(0))
	for _, f := range diffs {
		baseFile := sourcegraph.TreeEntrySpec{RepoRev: ds.Base, Path: f.OrigName}
		headFile := sourcegraph.TreeEntrySpec{RepoRev: ds.Head, Path: f.NewName}
		for _, hunk_ := range f.FileDiffHunks {
			hunk := hunk_
			par.Do(func() error {
				return formatFileDiffHunk(ctx, baseFile, headFile, hunk)
			})
		}
	}
	return par.Wait()
}

func formatFileDiffHunk(ctx context.Context, baseFile, headFile sourcegraph.TreeEntrySpec, hunk *sourcegraph.Hunk) error {
	ops := chunkDiffOps(baseFile, headFile, hunk)
	var fmtBody []byte
	for _, op := range ops {
		file, err := svc.RepoTree(ctx).Get(ctx, &sourcegraph.RepoTreeGetOp{Entry: op.file, Opt: &op.opt})
		if err != nil {
			return err
		}

		// KLUDGE(beyang): svc.RepoTree(ctx).Get doesn't return a trailing newline
		// (unless the last line is empty) for most chunks, but it does when the
		// newline occurs at the very end of the file.
		nLines := op.opt.EndLine - op.opt.StartLine + 1
		if int64(bytes.Count(file.Contents, []byte{'\n'})) == int64(nLines) && file.Contents[len(file.Contents)-1] == byte('\n') {
			file.Contents = file.Contents[:len(file.Contents)-1]
		}

		fmtBody = append(fmtBody, file.Contents...)
		fmtBody = append(fmtBody, '\n') // note: this *might* add an extra trailing newline
	}

	var err error
	hunk.Body, err = setHunkLines(hunk.Body, fmtBody)
	if err != nil {
		return err
	}
	return nil
}

// repoTreeGetOp is returned by chunkDiffOps and represents the
// arguments for a single call to RepoTree.Get. It is used only
// for this purpose.
type repoTreeGetOp struct {
	file sourcegraph.TreeEntrySpec
	opt  sourcegraph.RepoTreeGetOptions
}

// chunkDiffOps takes a base and head of a diff and chunks
// consecutive lines from either base or head into multi-line
// runs. This reduces the number of calls to RepoTree.Get needed
// (instead of one per line, it's one per consecutive run).
func chunkDiffOps(baseFile, headFile sourcegraph.TreeEntrySpec, hunk *sourcegraph.Hunk) []*repoTreeGetOp {
	var ops []*repoTreeGetOp
	lines := bytes.SplitAfter(hunk.Body, []byte{'\n'})
	var indexOrig, indexNew int32 // count how many lines from each ver's start line
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}

		// Additions only increment the line count for the new
		// file; deletions only increment the line count for
		// the orig file.
		if line[0] == '+' {
			indexNew++
		} else if line[0] == '-' {
			indexOrig++
		} else {
			indexNew++
			indexOrig++
		}

		// The file is set below based on whether whether this is from
		// the base or head.
		var file sourcegraph.TreeEntrySpec

		opt := sourcegraph.RepoTreeGetOptions{
			Formatted: true,
		}

		if line[0] == '+' {
			// Fetch line from head.
			file = headFile
			opt.StartLine = int64(hunk.NewStartLine + indexNew - 1)
			opt.EndLine = opt.StartLine
		} else {
			// Fetch line from base.
			file = baseFile
			opt.StartLine = int64(hunk.OrigStartLine + indexOrig - 1)
			opt.EndLine = opt.StartLine
		}

		// Consecutive run, or need to make a new op?
		if len(ops) == 0 || ops[len(ops)-1].file != file {
			// New op.
			ops = append(ops, &repoTreeGetOp{file, opt})
		} else {
			// Extend current op by 1 line.
			op := ops[len(ops)-1]
			op.opt.EndLine = opt.EndLine
		}
	}
	return ops
}

// setHunkLines replaces the lines in origBody (an original hunk body)
// with the corresponding (same-indexed) lines in fmtBody. The first
// character on each line of origBody (i.e., a ' ', '-', or '+') is
// retained in the returned bytes.
func setHunkLines(origBody, fmtBody []byte) ([]byte, error) {
	origLines := bytes.SplitAfter(origBody, []byte{'\n'})
	fmtLines := bytes.SplitAfter(fmtBody, []byte{'\n'})

	// KLUDGE(beyang): if the original body does not have a trailing new line, then the
	// formatted body should not either
	if len(fmtLines) == len(origLines)+1 && len(fmtLines[len(fmtLines)-1]) == 0 {
		fmtLines = fmtLines[:len(fmtLines)-1]
	}

	if len(fmtLines) != len(origLines) {
		return nil, fmt.Errorf("number of lines in original code does not equal number in formatted code (%d != %d)", len(origLines), len(fmtLines))
	}

	var merged []byte
	for i, origLine := range origLines {
		fmtLine := fmtLines[i]
		if len(origLine) > 0 {
			merged = append(merged, origLine[0])
		} else {
			merged = append(merged, ' ')
		}
		merged = append(merged, fmtLine...)
	}
	return merged, nil
}
