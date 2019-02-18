package repos

import (
	"time"

	"github.com/sourcegraph/sourcegraph/pkg/api"
)

// Repo represents a source code repository stored in Sourcegraph.
type Repo struct {
	// The internal Sourcegraph repo ID.
	ID uint32
	// Name is the name for this repository (e.g., "github.com/user/repo").
	//
	// Previously, this was called RepoURI.
	Name string
	// CloneURL is the clone URL of the repo.
	CloneURL string
	// Description is a brief description of the repository.
	Description string
	// Language is the primary programming language used in this repository.
	Language string
	// Fork is whether this repository is a fork of another repository.
	Fork bool
	// Enabled is whether the repository is enabled. Disabled repositories are
	// not accessible by users (except site admins).
	Enabled bool
	// Archived is whether the repository has been archived.
	Archived bool
	// CreatedAt is when this repository was created on Sourcegraph.
	CreatedAt time.Time
	// UpdatedAt is when this repository's metadata was last updated on Sourcegraph.
	UpdatedAt time.Time
	// DeletedAt is when this repository was soft-deleted from Sourcegraph.
	DeletedAt time.Time
	// ExternalRepo identifies this repository by its ID on the external service where it resides (and the external
	// service itself).
	ExternalRepo api.ExternalRepoSpec
	// Sources identifies all the source IDs this Repo belongs to.
	Sources []string
	// Metadata contains the raw source code host JSON metadata.
	Metadata []byte
}

// IDs returns the globally unique identifiers of a repository.
func (r *Repo) IDs() (ids []string) {
	ids = append(ids, r.Name)
	if extID := r.ExternalRepo.ServiceID + r.ExternalRepo.ID; extID != "" {
		ids = append(ids, extID)
	}
	return ids
}

// Clone returns a clone of the given repo.
func (r *Repo) Clone() *Repo {
	clone := *r
	return &clone
}

// Apply applies the given functional options to the Repo.
func (r *Repo) Apply(opts ...func(*Repo)) {
	if r == nil {
		return
	}

	for _, opt := range opts {
		opt(r)
	}
}

// With returns a clone of the given repo with the given functional options applied.
func (r *Repo) With(opts ...func(*Repo)) *Repo {
	clone := r.Clone()
	clone.Apply(opts...)
	return clone
}

// Repos is an utility type with convenience methods for operating on lists of Repos.
type Repos []*Repo

// IDs returns the list of IDs from all Repos.
func (rs Repos) IDs() []string {
	ids := make([]string, 0, len(rs)*2)
	for _, r := range rs {
		ids = append(ids, r.IDs()...)
	}
	return ids
}

// Names returns the list of names from all Repos.
func (rs Repos) Names() []string {
	names := make([]string, len(rs))
	for i := range rs {
		names[i] = rs[i].Name
	}
	return names
}