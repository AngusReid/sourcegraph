package graphqlbackend

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"time"

	graphql "github.com/graph-gophers/graphql-go"
	"github.com/pkg/errors"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/backend"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/db"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/pkg/discussions"
	"github.com/sourcegraph/sourcegraph/pkg/conf"
	"github.com/sourcegraph/sourcegraph/pkg/types"
)

var errDiscussionsNotEnabled = errors.New("discussions are not enabled on this site")

// marshalDiscussionID marshals a discussion thread or comment ID into a
// graphql.ID. These IDs are a lot like GitHub issue IDs: we want them to be
// pretty integer values (not base64 encoded values), so we just turn the ID
// integer into a string. Note we cannot use a GraphQL Int type, as it is not
// 64 bits.
func marshalDiscussionID(discussionID int64) graphql.ID {
	return graphql.ID(strconv.FormatInt(discussionID, 10))
}

func unmarshalDiscussionID(id graphql.ID) (discussionID int64, err error) {
	return strconv.ParseInt(string(id), 10, 64)
}

type discussionsMutationResolver struct {
}

type discussionThreadTargetRepoSelectionInput struct {
	StartLine      int32
	StartCharacter int32
	EndLine        int32
	EndCharacter   int32
	LinesBefore    *[]string
	Lines          *[]string
	LinesAfter     *[]string
}

type discussionThreadTargetRepoInput struct {
	Repository graphql.ID
	Path       *string
	Branch     *string
	Revision   *string
	Selection  *discussionThreadTargetRepoSelectionInput
}

func (d *discussionThreadTargetRepoInput) convert(ctx context.Context) (*types.DiscussionThreadTargetRepo, error) {
	repoID, err := unmarshalRepositoryID(d.Repository)
	if err != nil {
		return nil, err
	}
	tr := &types.DiscussionThreadTargetRepo{
		RepoID:   repoID,
		Path:     d.Path,
		Branch:   d.Branch,
		Revision: d.Revision,
	}
	if d.Selection != nil {
		tr.StartLine = &d.Selection.StartLine
		tr.EndLine = &d.Selection.EndLine
		tr.StartCharacter = &d.Selection.StartCharacter
		tr.EndCharacter = &d.Selection.EndCharacter

		if d.Selection.Lines == nil {
			// The caller wishes for us to populate the lines using repository
			// data. We do this now.
			if err := d.populateLinesFromRepository(ctx); err != nil {
				return nil, err
			}
		}
		tr.LinesBefore = d.Selection.LinesBefore
		tr.Lines = d.Selection.Lines
		tr.LinesAfter = d.Selection.LinesAfter
	}
	return tr, err
}

// validate checks the validity of the input and returns an error, if any.
func (d *discussionThreadTargetRepoInput) validate() error {
	if d.Selection != nil {
		// Check that the caller either specified all line fields or didn't specify
		// any at all (specifying some but not others makes no sense, see the
		// schema for details).
		equal := func(a, b, c bool) bool {
			return a != b || b != c
		}
		if ds := d.Selection; equal(ds.LinesBefore != nil, ds.Lines != nil, ds.LinesAfter != nil) {
			return errors.New("DiscussionThreadTargetRepoSelectionInput: linesBefore, lines, and linesAfter must all be null or non-null (not mixed)")
		}
		if d.Selection.Lines == nil {
			if d.Path == nil {
				return errors.New("DiscussionThreadTargetRepoSelectionInput: when lines are null, path field must be specified")
			}
			if d.Branch == nil && d.Revision == nil {
				return errors.New("DiscussionThreadTargetRepoSelectionInput: when lines are null, branch or revision field must be specified")
			}
		}
	}
	return nil
}

// populateLinesFromRepository populates the d.LinesBefore, d.Lines and
// d.LinesAfter fields by pulling the information directly from the repository.
//
// Precondition: d.Selection != nil && d.validate() == nil
func (d *discussionThreadTargetRepoInput) populateLinesFromRepository(ctx context.Context) error {
	if d.Selection == nil {
		panic("precondition failed")
	}
	repo, err := repositoryByID(ctx, d.Repository)
	if err != nil {
		return err
	}

	// First we must get the commit resolver with whichever revision is more
	// precise (branches can change revisions).
	var rev string
	if d.Revision != nil {
		rev = *d.Revision
	} else if d.Branch != nil {
		rev = *d.Branch
	} else {
		panic("precondition failed (protected by validation)")
	}
	commit, err := repo.Commit(ctx, &repositoryCommitArgs{Rev: rev})
	if err != nil {
		return err
	}

	// Now we can actually get the file content.
	if d.Path == nil {
		panic("precondition failed (protected by validation)")
	}
	blob, err := commit.Blob(ctx, &struct{ Path string }{Path: *d.Path})
	if err != nil {
		return err
	}
	fileContent, err := blob.Content(ctx)
	if err != nil {
		return err
	}

	// Grab the lines for the selection, populate the struct, and we're finished.
	linesBefore, lines, linesAfter := discussions.LinesForSelection(fileContent, discussions.LineRange{
		StartLine: int(d.Selection.StartLine),
		EndLine:   int(d.Selection.EndLine),
	})
	d.Selection.LinesBefore = &linesBefore
	d.Selection.Lines = &lines
	d.Selection.LinesAfter = &linesAfter
	return nil
}

func (r *discussionsMutationResolver) CreateThread(ctx context.Context, args *struct {
	Input *struct {
		Title      string
		Contents   string
		TargetRepo *discussionThreadTargetRepoInput
	}
}) (*discussionThreadResolver, error) {
	// 🚨 SECURITY: Only signed in users may create discussion threads.
	currentUser, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	if currentUser == nil {
		return nil, errors.New("no current user")
	}

	// Create the thread.
	newThread := &types.DiscussionThread{
		AuthorUserID: currentUser.user.ID,
		Title:        args.Input.Title,
	}
	if args.Input.TargetRepo != nil {
		if err := args.Input.TargetRepo.validate(); err != nil {
			return nil, err
		}
		newThread.TargetRepo, err = args.Input.TargetRepo.convert(ctx)
		if err != nil {
			return nil, err
		}
	}
	thread, err := db.DiscussionThreads.Create(ctx, newThread)
	if err != nil {
		return nil, errors.Wrap(err, "DiscussionThreads.Create")
	}

	// Create the first comment in the thread.
	newComment := &types.DiscussionComment{
		ThreadID:     newThread.ID,
		AuthorUserID: currentUser.user.ID,
		Contents:     args.Input.Contents,
	}
	_, err = db.DiscussionComments.Create(ctx, newComment)
	if err != nil {
		return nil, errors.Wrap(err, "DiscussionComments.Create")
	}
	discussions.NotifyNewThread(newThread, newComment)
	return &discussionThreadResolver{t: thread}, nil
}

func (r *discussionsMutationResolver) UpdateThread(ctx context.Context, args *struct {
	Input *struct {
		ThreadID graphql.ID
		Archive  *bool
		Delete   *bool
	}
}) (*discussionThreadResolver, error) {
	// 🚨 SECURITY: Only signed in users may update a discussion thread.
	currentUser, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	if currentUser == nil {
		return nil, errors.New("no current user")
	}

	threadID, err := unmarshalDiscussionID(args.Input.ThreadID)
	if err != nil {
		return nil, err
	}
	thread, err := db.DiscussionThreads.Update(ctx, threadID, &db.DiscussionThreadsUpdateOptions{
		Archive: args.Input.Archive,
	})
	if err != nil {
		return nil, errors.Wrap(err, "DiscussionThreads.Update")
	}

	if args.Input.Delete != nil {
		// 🚨 SECURITY: Only site admins can delete discussion threads.
		if err := backend.CheckCurrentUserIsSiteAdmin(ctx); err != nil {
			return nil, err
		}
		if err := db.DiscussionThreads.Delete(ctx, threadID); err != nil {
			return nil, errors.Wrap(err, "DiscussionThreads.Delete")
		}
		return nil, nil
	}
	return &discussionThreadResolver{t: thread}, nil
}

func (s *schemaResolver) Discussions(ctx context.Context) (*discussionsMutationResolver, error) {
	if !conf.DiscussionsEnabled() {
		return nil, errDiscussionsNotEnabled
	}
	return &discussionsMutationResolver{}, nil
}

func (s *schemaResolver) DiscussionThreads(ctx context.Context, args *struct {
	connectionArgs
	Query                *string
	ThreadID             *graphql.ID
	AuthorUserID         *graphql.ID
	TargetRepositoryID   *graphql.ID
	TargetRepositoryPath *string
}) (*discussionThreadsConnectionResolver, error) {
	if !conf.DiscussionsEnabled() {
		return nil, errDiscussionsNotEnabled
	}

	// 🚨 SECURITY: No authentication is required to list discussions. They are
	// public unless the Sourcegraph instance itself (and inherently, the
	// GraphQL API) is private.

	opt := &db.DiscussionThreadsListOptions{
		TargetRepoPath: args.TargetRepositoryPath,
		TitleQuery:     args.Query,
	}
	args.connectionArgs.set(&opt.LimitOffset)

	if args.ThreadID != nil {
		threadID, err := unmarshalDiscussionID(*args.ThreadID)
		if err != nil {
			return nil, err
		}
		opt.ThreadID = &threadID
	}
	if args.AuthorUserID != nil {
		authorUserID, err := unmarshalUserID(*args.AuthorUserID)
		if err != nil {
			return nil, err
		}
		opt.AuthorUserID = &authorUserID
	}
	if args.TargetRepositoryID != nil {
		repositoryID, err := unmarshalRepositoryID(*args.TargetRepositoryID)
		if err != nil {
			return nil, err
		}
		opt.TargetRepoID = &repositoryID
	}
	return &discussionThreadsConnectionResolver{opt: opt}, nil
}

type discussionThreadTargetRepoSelectionResolver struct {
	t *types.DiscussionThreadTargetRepo
}

func (r *discussionThreadTargetRepoSelectionResolver) StartLine() int32 { return *r.t.StartLine }
func (r *discussionThreadTargetRepoSelectionResolver) StartCharacter() int32 {
	return *r.t.StartCharacter
}
func (r *discussionThreadTargetRepoSelectionResolver) EndLine() int32        { return *r.t.EndLine }
func (r *discussionThreadTargetRepoSelectionResolver) EndCharacter() int32   { return *r.t.EndCharacter }
func (r *discussionThreadTargetRepoSelectionResolver) LinesBefore() []string { return *r.t.LinesBefore }
func (r *discussionThreadTargetRepoSelectionResolver) Lines() []string       { return *r.t.Lines }
func (r *discussionThreadTargetRepoSelectionResolver) LinesAfter() []string  { return *r.t.LinesAfter }

type discussionThreadTargetRepoResolver struct {
	t *types.DiscussionThreadTargetRepo
}

func (r *discussionThreadTargetRepoResolver) Repository(ctx context.Context) (*repositoryResolver, error) {
	return repositoryByIDInt32(ctx, r.t.RepoID)
}

func (r *discussionThreadTargetRepoResolver) Path() *string { return r.t.Path }

func (r *discussionThreadTargetRepoResolver) Branch(ctx context.Context) (*gitRefResolver, error) {
	return r.branchOrRevision(ctx, r.t.Branch)
}

func (r *discussionThreadTargetRepoResolver) Revision(ctx context.Context) (*gitRefResolver, error) {
	return r.branchOrRevision(ctx, r.t.Revision)
}

func (r *discussionThreadTargetRepoResolver) branchOrRevision(ctx context.Context, rev *string) (*gitRefResolver, error) {
	if rev == nil {
		return nil, nil
	}
	repo, err := repositoryByIDInt32(ctx, r.t.RepoID)
	if err != nil {
		return nil, err
	}
	return &gitRefResolver{repo: repo, name: *rev}, nil
}

func (r *discussionThreadTargetRepoResolver) Selection() *discussionThreadTargetRepoSelectionResolver {
	if !r.t.HasSelection() {
		return nil
	}
	return &discussionThreadTargetRepoSelectionResolver{t: r.t}
}

func (r *discussionThreadTargetRepoResolver) RelativePath(ctx context.Context, args *struct {
	Rev string
}) (*string, error) {
	if r.t.Path == nil {
		return nil, nil
	}
	repo, err := repositoryByIDInt32(ctx, r.t.RepoID)
	if err != nil {
		return nil, err
	}
	if r.t.Revision == nil && r.t.Branch == nil {
		// The thread wasn't created on a specific revision or branch, so we
		// cannot walk the history. Instead, we must assume its location and
		// check in the relative revision.
		commit, err := repo.Commit(ctx, &repositoryCommitArgs{Rev: args.Rev})
		if err != nil {
			return nil, err
		}
		_, err = commit.File(ctx, &struct{ Path string }{Path: *r.t.Path})
		if err != nil {
			// File does not exist in this revision.
			return nil, nil
		}
		return r.t.Path, nil // File exists at that path.
	}

	var rev string
	if r.t.Revision != nil {
		rev = *r.t.Revision
	} else if r.t.Branch != nil {
		rev = *r.t.Branch
	}
	comparison, err := repo.Comparison(ctx, &repositoryComparisonInput{
		Base: &rev,
		Head: &args.Rev,
	})
	if err != nil {
		return nil, err
	}
	currentPath := *r.t.Path
	fileDiffs, err := comparison.FileDiffs(&struct{ First *int32 }{}).Nodes(ctx)
	if err != nil {
		return nil, err
	}
	for _, fileDiff := range fileDiffs {
		oldPath := fileDiff.OldPath()
		newPath := fileDiff.NewPath()

		if oldPath == nil && newPath != nil {
			// newPath was added. We don't need to do anything because this
			// could only indicate the file we're tracking was added.
		} else if oldPath != nil && newPath == nil {
			// oldPath was removed
			if currentPath == *oldPath {
				// The file we are tracking was removed!
				return nil, nil
			}
		} else if oldPath != nil && newPath != nil {
			// oldPath was renamed to newPath
			if currentPath == *oldPath {
				// The file we are tracking was renamed.
				currentPath = *newPath
			}
		}
	}
	return &currentPath, nil
}

type discussionSelectionRangeResolver struct {
	startLine, startCharacter, endLine, endCharacter int32
}

func (r *discussionSelectionRangeResolver) StartLine() int32      { return r.startLine }
func (r *discussionSelectionRangeResolver) StartCharacter() int32 { return r.startCharacter }
func (r *discussionSelectionRangeResolver) EndLine() int32        { return r.endLine }
func (r *discussionSelectionRangeResolver) EndCharacter() int32   { return r.endCharacter }

func discussionSelectionRelativeTo(oldSel *types.DiscussionThreadTargetRepo, newContent string) *discussionSelectionRangeResolver {
	mustFindLines := 4

	search := func(searchForLines string) *discussionSelectionRangeResolver {
		if len(strings.Split(searchForLines, "\n")) < mustFindLines {
			// We do not have enough search lines to find a good match.
			return nil
		}
		matches := strings.Count(newContent, searchForLines)
		switch {
		case matches > 1:
			// The lines we are searching for produced too many matches.
			return nil
		case matches == 1:
			// We found a perfect match.
			idx := strings.Index(newContent, searchForLines)
			startLine := int32(len(strings.Split(newContent[:idx], "\n")))
			return &discussionSelectionRangeResolver{
				startCharacter: *oldSel.StartCharacter,
				endCharacter:   *oldSel.EndCharacter,
				startLine:      startLine,
				endLine:        startLine + int32(len(*oldSel.Lines)),
			}
		default:
			return nil
		}
	}

	// Start removing lines until we find a result (or fail to find one).
	allLines := *oldSel.LinesBefore
	allLines = append(allLines, *oldSel.Lines...)
	allLines = append(allLines, *oldSel.LinesAfter...)
	removeLines := 0
	for {
		if removeLines > len(allLines) {
			return nil
		}
		// Try removing N lines from the top.
		if r := search(strings.Join(allLines[removeLines:], "\n")); r != nil {
			offset := int32(len(*oldSel.LinesBefore) - 1 - removeLines)
			r.startLine += offset
			r.endLine += offset
			return r
		}

		// Try removing N lines from the bottom.
		if r := search(strings.Join(allLines[:len(allLines)-removeLines], "\n")); r != nil {
			offset := int32(len(*oldSel.LinesAfter) - 1 - removeLines)
			r.startLine += offset
			r.endLine += offset
			return r
		}
		removeLines++
	}
}

func (r *discussionThreadTargetRepoResolver) RelativeSelection(ctx context.Context, args *struct {
	Rev string
}) (*discussionSelectionRangeResolver, error) {
	if !r.t.HasSelection() {
		return nil, nil
	}
	path, err := r.RelativePath(ctx, args)
	if err != nil {
		return nil, err
	}
	if path == nil {
		return nil, nil
	}
	repo, err := repositoryByIDInt32(ctx, r.t.RepoID)
	if err != nil {
		return nil, err
	}
	commit, err := repo.Commit(ctx, &repositoryCommitArgs{Rev: args.Rev})
	if err != nil {
		return nil, err
	}
	oldSel := &discussionSelectionRangeResolver{
		startLine:      *r.t.StartLine,
		startCharacter: *r.t.StartCharacter,
		endLine:        *r.t.EndLine,
		endCharacter:   *r.t.EndCharacter,
	}
	if r.t.Revision != nil && *r.t.Revision == string(commit.OID()) {
		return oldSel, nil // nothing to do (requested relative revision is identical to the stored revision)
	}
	if r.t.Branch != nil {
		branchCommit, err := repo.Commit(ctx, &repositoryCommitArgs{Rev: *r.t.Branch})
		if err != nil {
			return nil, err
		}
		if branchCommit.OID() == commit.OID() {
			return oldSel, nil // nothing to do (requested relative revision is identical to the stored branch revision)
		}
	}
	file, err := commit.File(ctx, &struct{ Path string }{Path: *path})
	if err != nil {
		return nil, err
	}
	newContent, err := file.Content(ctx)
	if err != nil {
		return nil, err
	}
	return discussionSelectionRelativeTo(r.t, newContent), nil
}

type discussionThreadTargetResolver struct {
	t *types.DiscussionThread
}

func (r *discussionThreadTargetResolver) ToDiscussionThreadTargetRepo() (*discussionThreadTargetRepoResolver, bool) {
	if r.t.TargetRepo == nil {
		return nil, false
	}
	return &discussionThreadTargetRepoResolver{t: r.t.TargetRepo}, true
}

// 🚨 SECURITY: When instantiating an discussionThreadResolver value, the
// caller MUST check permissions.
type discussionThreadResolver struct {
	t *types.DiscussionThread
}

func (d *discussionThreadResolver) ID() graphql.ID {
	return marshalDiscussionID(d.t.ID)
}

func (d *discussionThreadResolver) Author(ctx context.Context) (*userResolver, error) {
	return userByIDInt32(ctx, d.t.AuthorUserID)
}

func (d *discussionThreadResolver) Title() string { return d.t.Title }

func (d *discussionThreadResolver) Target(ctx context.Context) *discussionThreadTargetResolver {
	return &discussionThreadTargetResolver{t: d.t}
}

func (d *discussionThreadResolver) InlineURL(ctx context.Context) (*string, error) {
	url, err := discussions.URLToInlineThread(ctx, d.t)
	if err != nil {
		return nil, err
	}
	return strptr(url.String()), nil
}

func (d *discussionThreadResolver) CreatedAt(ctx context.Context) string {
	return d.t.CreatedAt.Format(time.RFC3339)
}

func (d *discussionThreadResolver) UpdatedAt(ctx context.Context) string {
	return d.t.UpdatedAt.Format(time.RFC3339)
}

func (d *discussionThreadResolver) ArchivedAt(ctx context.Context) *string {
	if d.t.ArchivedAt == nil {
		return nil
	}
	return strptr(d.t.ArchivedAt.Format(time.RFC3339))
}

func (d *discussionThreadResolver) Comments(ctx context.Context, args *struct {
	connectionArgs
}) *discussionCommentsConnectionResolver {
	// 🚨 SECURITY: Anyone with access to the thread also has access to its
	// comments. Hence, since we are only accessing the threads comments here
	// (and not other thread's comments) we are covered security-wise here
	// implicitly.

	opt := &db.DiscussionCommentsListOptions{ThreadID: &d.t.ID}
	args.connectionArgs.set(&opt.LimitOffset)
	return &discussionCommentsConnectionResolver{opt: opt}
}

// discussionThreadsConnectionResolver resolves a list of discussion comments.
//
// 🚨 SECURITY: When instantiating an discussionThreadsConnectionResolver
// value, the caller MUST check permissions.
type discussionThreadsConnectionResolver struct {
	opt *db.DiscussionThreadsListOptions

	// cache results because they are used by multiple fields
	once     sync.Once
	comments []*types.DiscussionThread
	err      error
}

func (r *discussionThreadsConnectionResolver) compute(ctx context.Context) ([]*types.DiscussionThread, error) {
	r.once.Do(func() {
		opt2 := *r.opt
		if opt2.LimitOffset != nil {
			tmp := *opt2.LimitOffset
			opt2.LimitOffset = &tmp
			opt2.Limit++ // so we can detect if there is a next page
		}

		r.comments, r.err = db.DiscussionThreads.List(ctx, &opt2)
	})
	return r.comments, r.err
}

func (r *discussionThreadsConnectionResolver) Nodes(ctx context.Context) ([]*discussionThreadResolver, error) {
	threads, err := r.compute(ctx)
	if err != nil {
		return nil, err
	}
	var l []*discussionThreadResolver
	for _, thread := range threads {
		l = append(l, &discussionThreadResolver{t: thread})
	}
	return l, nil
}

func (r *discussionThreadsConnectionResolver) TotalCount(ctx context.Context) (int32, error) {
	withoutLimit := *r.opt
	withoutLimit.LimitOffset = nil
	count, err := db.DiscussionThreads.Count(ctx, &withoutLimit)
	return int32(count), err
}

func (r *discussionThreadsConnectionResolver) PageInfo(ctx context.Context) (*pageInfo, error) {
	threads, err := r.compute(ctx)
	if err != nil {
		return nil, err
	}
	return &pageInfo{hasNextPage: r.opt.LimitOffset != nil && len(threads) > r.opt.Limit}, nil
}