//docker:user sourcegraph

// Command repo-updater periodically updates repositories configured in site configuration and serves repository
// metadata from multiple external code hosts.
package main

import (
	"context"
	"log"
	"net/http"

	"github.com/opentracing-contrib/go-stdlib/nethttp"
	opentracing "github.com/opentracing/opentracing-go"
	"gopkg.in/inconshreveable/log15.v2"

	"github.com/sourcegraph/sourcegraph/cmd/repo-updater/repos"
	"github.com/sourcegraph/sourcegraph/cmd/repo-updater/repoupdater"
	"github.com/sourcegraph/sourcegraph/pkg/api"
	"github.com/sourcegraph/sourcegraph/pkg/debugserver"
	"github.com/sourcegraph/sourcegraph/pkg/env"
	"github.com/sourcegraph/sourcegraph/pkg/tracer"
)

func main() {
	ctx := context.Background()
	env.Lock()
	env.HandleHelpFlag()
	tracer.Init()

	go debugserver.Start()

	// Start up handler that frontend relies on
	var repoupdater repoupdater.Server
	// Log usage statistics
	go repoupdater.RecordStats()
	handler := nethttp.Middleware(opentracing.GlobalTracer(), repoupdater.Handler())
	log15.Info("repo-updater: listening", "addr", ":3182")
	srv := &http.Server{Addr: ":3182", Handler: handler}
	go func() { log.Fatal(srv.ListenAndServe()) }()

	// Sync relies on access to frontend, so wait until it has started up.
	api.WaitForFrontend(ctx)

	// Repos List syncing thread
	go repos.RunRepositorySyncWorker(ctx)

	// Repos purging thread
	go repos.RunRepositoryPurgeWorker(ctx)

	// GitHub Repository syncing thread
	go repos.RunGitHubRepositorySyncWorker(ctx)

	// GitLab Repository syncing thread
	go repos.RunGitLabRepositorySyncWorker(ctx)

	// AWS CodeCommit repository syncing thread
	go repos.RunAWSCodeCommitRepositorySyncWorker(ctx)

	// Phabricator Repository syncing thread
	go repos.RunPhabricatorRepositorySyncWorker(ctx)

	// Gitolite syncing thread
	go repos.RunGitoliteRepositorySyncWorker(ctx)

	// Bitbucket Server syncing thread
	go repos.RunBitbucketServerRepositorySyncWorker(ctx)

	select {}
}