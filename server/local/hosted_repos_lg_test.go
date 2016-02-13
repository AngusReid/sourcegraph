// +build exectest

package local_test

import (
	"testing"

	"src.sourcegraph.com/sourcegraph/auth/authutil"
	"src.sourcegraph.com/sourcegraph/server/testserver"
	"src.sourcegraph.com/sourcegraph/util/httptestutil"
	"src.sourcegraph.com/sourcegraph/util/testutil"
)

var httpClient = &httptestutil.Client{}

func TestHostedRepo_CreateCloneAndView(t *testing.T) {
	a, ctx := testserver.NewUnstartedServer()
	a.Config.ServeFlags = append(a.Config.ServeFlags,
		&authutil.Flags{Source: "none", AllowAnonymousReaders: true},
	)
	if err := a.Start(); err != nil {
		t.Fatal(err)
	}
	defer a.Close()

	_, _, done, err := testutil.CreateAndPushRepo(t, ctx, "r/r")
	if err != nil {
		t.Fatal(err)
	}
	defer done()

	// TODO(sqs): also test when there are no commits that we show a
	// "no commits yet" page, and same for when there are no files.

	if _, err := httpClient.GetOK(a.AbsURL("/r/r")); err != nil {
		t.Fatal(err)
	}
}
