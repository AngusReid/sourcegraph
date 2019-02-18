package repos

import (
	"bytes"
	"context"
	"sort"
	"time"

	multierror "github.com/hashicorp/go-multierror"
	"github.com/sourcegraph/sourcegraph/pkg/api"
	log15 "gopkg.in/inconshreveable/log15.v2"
)

// A Syncer periodically synchronizes available repositories from all its given Sources
// with the stored Repositories in Sourcegraph.
type Syncer struct {
	interval time.Duration
	store    Store
	sourcer  Sourcer
	diffs    chan Diff
	now      func() time.Time
}

// NewSyncer returns a new Syncer that periodically synchronizes stored repos with
// the repos yielded by the configured sources, retrieved by the given sourcer.
// Each completed sync results in a diff that is sent to the given diffs channel.
func NewSyncer(
	interval time.Duration,
	store Store,
	sourcer Sourcer,
	diffs chan Diff,
	now func() time.Time,
) *Syncer {
	return &Syncer{
		interval: interval,
		store:    store,
		sourcer:  sourcer,
		diffs:    diffs,
		now:      now,
	}
}

// Run runs the Sync at its specified interval.
func (s Syncer) Run(ctx context.Context) error {
	for ctx.Err() == nil {
		if _, err := s.Sync(ctx); err != nil {
			log15.Error("Syncer", "err", err)
		}
		time.Sleep(s.interval)
	}

	return ctx.Err()
}

// Sync synchronizes the repositories of a single Source
func (s Syncer) Sync(ctx context.Context) (_ Diff, err error) {
	// TODO(tsenart): Ensure that transient failures do not remove
	// repositories. This means we need to use the store as a fallback Source
	// in the face of those kinds of errors, so that the diff results in Unmodified
	// entries. This logic can live here. We only need to make the returned error
	// more structured so we can identify which sources failed and for what reason.
	// See the SyncError type defined in other_external_services.go for inspiration.

	var sourced Repos
	if sourced, err = s.sourced(ctx); err != nil {
		return Diff{}, err
	}

	store := s.store
	if tr, ok := s.store.(Transactor); ok {
		var txs TxStore
		if txs, err = tr.Transact(ctx); err != nil {
			return Diff{}, err
		}
		defer txs.Done(&err)
		store = txs
	}

	var stored Repos
	if stored, err = store.ListRepos(ctx); err != nil {
		return Diff{}, err
	}

	diff := s.diff(sourced, stored)
	upserts := s.upserts(diff)

	if err = store.UpsertRepos(ctx, upserts...); err != nil {
		return Diff{}, err
	}

	if s.diffs != nil {
		s.diffs <- diff
	}

	return diff, nil
}

func (s Syncer) upserts(diff Diff) []*Repo {
	now := s.now()
	upserts := make([]*Repo, 0, len(diff.Added)+len(diff.Deleted)+len(diff.Modified))

	for _, add := range diff.Added {
		repo := add.(*Repo)
		repo.CreatedAt, repo.DeletedAt = now, time.Time{}
		upserts = append(upserts, repo)
	}

	for _, mod := range diff.Modified {
		repo := mod.(*Repo)
		repo.UpdatedAt, repo.DeletedAt = now, time.Time{}
		upserts = append(upserts, repo)
	}

	for _, del := range diff.Deleted {
		repo := del.(*Repo)
		repo.UpdatedAt, repo.DeletedAt = now, now
		repo.Sources = []string{}
		upserts = append(upserts, repo)
	}

	return upserts
}

func (Syncer) diff(sourced, stored []*Repo) Diff {
	before := make([]Diffable, len(stored))
	for i := range stored {
		before[i] = stored[i]
	}

	after := make([]Diffable, len(sourced))
	for i := range sourced {
		after[i] = sourced[i]
	}

	return NewDiff(before, after, func(before, after Diffable) bool {
		// This modified function returns true iff any fields in `after` changed
		// in comparison to `before` for which the `Source` is authoritative.
		b, a := before.(*Repo), after.(*Repo)
		return b.Name != a.Name ||
			b.Language != a.Language ||
			b.Fork != a.Fork ||
			b.Archived != a.Archived ||
			b.Description != a.Description ||
			// Only update the external id once. It should not change after it's set.
			(b.ExternalRepo == api.ExternalRepoSpec{} &&
				b.ExternalRepo != a.ExternalRepo) ||
			!equal(b.Sources, a.Sources) ||
			!bytes.Equal(b.Metadata, a.Metadata)
	})
}

func (s Syncer) sourced(ctx context.Context) ([]*Repo, error) {
	sources, err := s.sourcer.ListSources(ctx)
	if err != nil {
		return nil, err
	}

	type result struct {
		src   Source
		repos []*Repo
		err   error
	}

	ch := make(chan result, len(sources))
	for _, src := range sources {
		go func(src Source) {
			if repos, err := src.ListRepos(ctx); err != nil {
				ch <- result{src: src, err: err}
			} else {
				ch <- result{src: src, repos: repos}
			}
		}(src)
	}

	set := make(map[string]*Repo)
	var repos []*Repo
	var errs *multierror.Error

	for i := 0; i < cap(ch); i++ {
		r := <-ch

		if r.err != nil {
			errs = multierror.Append(errs, r.err)
			continue
		}

		for _, repo := range r.repos {
			for _, id := range repo.IDs() {
				if existing, ok := set[id]; ok {
					merge(existing, repo)
				} else {
					set[id] = repo
					repos = append(repos, repo)
				}
			}
		}
	}

	return repos, errs.ErrorOrNil()
}

// Merge two instances of the same Repo that were yielded
// by different Sources.
func merge(a, b *Repo) {
	// If we got rate limited, let's preserve the previous external id
	// which should be stable and never change.
	if a.ExternalRepo == (api.ExternalRepoSpec{}) {
		a.ExternalRepo = b.ExternalRepo
	}

	// TODO(tsenart): Extract an updated_at timestamp from the metadata
	// and use it to decide which repo has the most up-to-date information.

	srcs := make([]string, 0, len(a.Sources)+len(b.Sources))
	srcs = append(srcs, a.Sources...)
	srcs = append(srcs, b.Sources...)
	a.Sources = dedup(srcs...)
	sort.Strings(a.Sources)
}

func dedup(ss ...string) []string {
	uniq := make([]string, 0, len(ss))
	set := make(map[string]struct{}, len(ss))
	for _, s := range ss {
		if _, ok := set[s]; !ok {
			set[s] = struct{}{}
			uniq = append(uniq, s)
		}
	}
	return uniq
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}
