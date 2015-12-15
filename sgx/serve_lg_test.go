// +build exectest

package sgx_test

import (
	"crypto/tls"
	"flag"
	"log"
	"net/http"
	"net/url"
	"os"
	"testing"

	"golang.org/x/net/context"

	"strings"

	"src.sourcegraph.com/sourcegraph/auth/authutil"
	"src.sourcegraph.com/sourcegraph/server/testserver"
	"src.sourcegraph.com/sourcegraph/util/httptestutil"

	"sync"

	"sourcegraph.com/sqs/pbtypes"
	"src.sourcegraph.com/sourcegraph/conf"
)

// Test that spawning one server works (the simple case).
func TestServer(t *testing.T) {
	testServer(t)
}

// Test that spawning one TLS server works.
func TestServerTLS(t *testing.T) {
	t.Skip("flaky") // https://circleci.com/gh/sourcegraph/sourcegraph/9549

	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	defer func() {
		http.DefaultTransport.(*http.Transport).TLSClientConfig.InsecureSkipVerify = false
	}()

	a, ctx := testserver.NewUnstartedServerTLS()
	a.Config.Serve.RedirectToHTTPS = true

	doTestServer(t, a, ctx)
	defer a.Close()

	// Test that HTTP redirects to HTTPS.
	httpsURL := conf.AppURL(ctx).ResolveReference(&url.URL{Path: "/foo/bar"}).String()
	httpURL := strings.Replace(httpsURL, "https://", "http://", 1)
	httpURL = strings.Replace(httpURL, a.Config.Serve.HTTPSAddr, a.Config.Serve.HTTPAddr, 1)
	httpClient := &httptestutil.Client{}
	resp, err := httpClient.GetNoFollowRedirects(httpURL)
	if err != nil {
		t.Fatal(err)
	}
	if err := resp.Body.Close(); err != nil {
		t.Fatal(err)
	}
	if want := http.StatusMovedPermanently; resp.StatusCode != want {
		t.Errorf("got HTTP status %d, want %d", resp.StatusCode, want)
	}
	if got, want := resp.Header.Get("location"), strings.Replace(httpURL, "http://", "https://", 1); got != want {
		t.Errorf("got HTTP Location redirect to %q, want %q", got, want)
	}
}

var numServersSerialParallel = flag.Int("test.servers", 3, "number of servers to spawn for serial/parallel server tests")

// Test that spawning many servers serially works (and that random
// ports are chosen correctly, etc.).
//
// This is more a test of testserver.Server than package sgx, but it uses
// testServer, so it is convenient to put it here.
func TestManyServers_Serial(t *testing.T) {
	for i := 0; i < *numServersSerialParallel; i++ {
		t.Logf("serial server %d starting...", i)
		testServer(t)
		t.Logf("serial server %d ending", i)
	}
}

// Test that spawning many servers in parallel works (and that random
// ports are chosen correctly, etc.).
//
// This is more a test of testserver.Server than package sgx, but it uses
// testServer, so it is convenient to put it here.
func TestManyServers_Parallel(t *testing.T) {
	if os.Getenv("CI") != "" {
		// Failing on Travis CI
		t.Skip()
		return
	}

	var wg sync.WaitGroup
	for i := 0; i < *numServersSerialParallel; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			t.Logf("parallel server %d starting...", i)
			testServer(t)
			t.Logf("parallel server %d ended", i)
		}(i)
	}
	wg.Wait()
}

func testServer(t *testing.T) {
	a, ctx := testserver.NewUnstartedServer()
	doTestServer(t, a, ctx)
	defer a.Close()
}

func doTestServer(t *testing.T, a *testserver.Server, ctx context.Context) {
	a.Config.ServeFlags = append(a.Config.ServeFlags,
		&authutil.Flags{Source: "none", AllowAnonymousReaders: true},
	)

	if err := a.Start(); err != nil {
		log.Fatal(err)
	}

	// Test gRPC server.
	serverConfig, err := a.Client.Meta.Config(ctx, &pbtypes.Void{})
	if err != nil {
		t.Fatal(err)
	}

	// Test HTTP API.
	httpURL, err := url.Parse(serverConfig.AppURL)
	if err != nil {
		t.Fatal(err)
	}
	apiURL := httpURL.ResolveReference(&url.URL{Path: "/.api/.defs"}).String()
	resp, err := http.Get(apiURL)
	if err != nil {
		t.Fatal(err)
	}
	if err := resp.Body.Close(); err != nil {
		t.Fatal(err)
	}
	if want := http.StatusOK; resp.StatusCode != want {
		t.Errorf("got HTTP status %d, want %d", resp.StatusCode, want)
	}

	// Test app server.
	resp3, err := http.Get(conf.AppURL(ctx).String())
	if err != nil {
		t.Fatal(err)
	}
	if err := resp3.Body.Close(); err != nil {
		t.Fatal(err)
	}
	if want := http.StatusOK; resp3.StatusCode != want {
		t.Errorf("got HTTP status %d, want %d", resp3.StatusCode, want)
	}

	// Check config.
	if want := conf.AppURL(ctx).String(); serverConfig.AppURL != want {
		t.Errorf("got AppURL %q, want %q", serverConfig.AppURL, want)
	}

	if err := a.Cmd(nil, []string{"meta", "status"}).Run(); err != nil {
		t.Errorf("meta status cmd failed: %s", err)
	}
}
