package gohttpdisk

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/dnaeon/go-vcr/recorder"
	"github.com/stretchr/testify/assert"
)

func TestHTTPDisk(t *testing.T) {
	client := setupClient(t, Options{})
	defer teardownClient(client)

	drainBody := func(resp *http.Response) string {
		data, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			panic(err)
		}
		return string(data)
	}

	//
	// 1. miss
	//

	url := "http://example.com/get"
	resp, err := client.Get(url)
	if err != nil {
		t.Fatalf("Get %s failed %s", url, err)
	}
	defer resp.Body.Close()
	assert.Equal(t, "body 1", drainBody(resp))
	assert.Equal(t, "1", resp.Header.Get("X-Request-Id"))

	//
	// 2. hit
	//

	resp, err = client.Get(url)
	if err != nil {
		t.Fatalf("Get %s failed %s", url, err)
	}
	defer resp.Body.Close()
	assert.Equal(t, "body 1", drainBody(resp))
	assert.Equal(t, "1", resp.Header.Get("X-Request-Id"))
}

func TestHTTPDiskForce(t *testing.T) {
	client := setupClient(t, Options{Force: true})
	defer teardownClient(client)

	//
	// 1. miss
	//

	url := "http://example.com/get"
	resp, err := client.Get(url)
	if err != nil {
		t.Fatalf("Get %s failed %s", url, err)
	}
	defer resp.Body.Close()
	assert.Equal(t, "1", resp.Header.Get("X-Request-Id"))

	//
	// 2. force second request
	//

	resp, err = client.Get(url)
	if err != nil {
		t.Fatalf("Get %s failed %s", url, err)
	}
	defer resp.Body.Close()
	assert.Equal(t, "2", resp.Header.Get("X-Request-Id"))
}

func TestHTTPDiskErrors(t *testing.T) {
	client := setupClient(t, Options{})
	defer teardownClient(client)

	var resp *http.Response
	var err error

	// Nework errors are tested elsewhere. See TestHTTPDiskTimeout and TestHTTPDiskNoSuchHost.

	// 40x error
	url := "http://httpbingo.org/status/404"
	resp, err = client.Get(url)
	assert.Nil(t, err)
	assert.Equal(t, 404, resp.StatusCode)
	assert.Equal(t, "1", resp.Header.Get("X-Request-Id"))

	resp, err = client.Get(url)
	assert.Nil(t, err)
	assert.Equal(t, 404, resp.StatusCode)
	assert.Equal(t, "1", resp.Header.Get("X-Request-Id"), "response not cached")

	// 50x error
	url = "http://httpbingo.org/status/502"
	resp, err = client.Get(url)
	assert.Nil(t, err)
	assert.Equal(t, 502, resp.StatusCode)
	assert.Equal(t, "2", resp.Header.Get("X-Request-Id"))

	resp, err = client.Get(url)
	assert.Nil(t, err)
	assert.Equal(t, 502, resp.StatusCode)
	assert.Equal(t, "2", resp.Header.Get("X-Request-Id"), "response not cached")
}

func TestHTTPDiskTimeout(t *testing.T) {
	// This test does not use vcr
	hd := NewHTTPDisk(Options{Dir: TmpDir()})
	hd.Cache.RemoveAll()
	defer hd.Cache.RemoveAll()

	// Fake the error unless we're really hitting the network
	if os.Getenv("USE_NETWORK") == "" {
		hd.Transport = &errorRoundTripper{"context deadline exceeded"}
	}

	client := http.Client{Transport: hd, Timeout: 500 * time.Millisecond}

	url := "http://httpbingo.org/delay/1"
	_, err := client.Get(url)
	assert.NotNil(t, err)

	_, err = client.Get(url)
	if assert.NotNil(t, err) {
		assert.Contains(t, err.Error(), "(cached)", "%s error was not cached", url)
	}
}

func TestHTTPDiskNoSuchHost(t *testing.T) {
	// This test does not use vcr
	hd := NewHTTPDisk(Options{Dir: TmpDir()})
	hd.Cache.RemoveAll()
	defer hd.Cache.RemoveAll()

	// Fake the error unless we're really hitting the network
	if os.Getenv("USE_NETWORK") == "" {
		hd.Transport = &errorRoundTripper{"no such host"}
	}

	client := http.Client{Transport: hd}

	url := "http://bogus.bogus"
	_, err := client.Get(url)
	assert.NotNil(t, err)

	_, err = client.Get(url)
	if assert.NotNil(t, err) {
		assert.Contains(t, err.Error(), "(cached)", "%s error was not cached", url)
	}
}

func TestHTTPDiskForceErrors(t *testing.T) {
	client := setupClient(t, Options{ForceErrors: true})
	defer teardownClient(client)

	var resp *http.Response
	var err error

	// Nework errors are tested elsewhere. See TestHTTPDiskForceTimeout and TestHTTPDiskForceNoSuchHost.

	// 40x error
	url := "http://httpbingo.org/status/404"
	resp, err = client.Get(url)
	assert.Nil(t, err)
	assert.Equal(t, 404, resp.StatusCode)
	assert.Equal(t, "1", resp.Header.Get("X-Request-Id"))

	resp, err = client.Get(url)
	assert.Nil(t, err)
	assert.Equal(t, 404, resp.StatusCode)
	assert.Equal(t, "2", resp.Header.Get("X-Request-Id"), "response cached")

	// 50x error
	url = "http://httpbingo.org/status/502"
	resp, err = client.Get(url)
	assert.Nil(t, err)
	assert.Equal(t, 502, resp.StatusCode)
	assert.Equal(t, "3", resp.Header.Get("X-Request-Id"))

	resp, err = client.Get(url)
	assert.Nil(t, err)
	assert.Equal(t, 502, resp.StatusCode)
	assert.Equal(t, "4", resp.Header.Get("X-Request-Id"), "response cached")
}

func TestHTTPDiskForceTimeout(t *testing.T) {
	// This test does not use vcr
	hd := NewHTTPDisk(Options{Dir: TmpDir(), ForceErrors: true})
	hd.Cache.RemoveAll()
	defer hd.Cache.RemoveAll()

	// Fake the error unless we're really hitting the network
	if os.Getenv("USE_NETWORK") == "" {
		hd.Transport = &errorRoundTripper{"context deadline exceeded"}
	}

	client := http.Client{Transport: hd, Timeout: 500 * time.Millisecond}

	url := "http://httpbingo.org/delay/1"
	client.Get(url)
	_, err := client.Get(url)
	if assert.NotNil(t, err) {
		assert.NotContains(t, err.Error(), "(cached)", "%s ForceErrors not honored", url)
	}
}

func TestHTTPDiskForceNoSuchHost(t *testing.T) {
	// This test does not use vcr
	hd := NewHTTPDisk(Options{Dir: TmpDir(), ForceErrors: true})
	hd.Cache.RemoveAll()
	defer hd.Cache.RemoveAll()

	// Fake the error unless we're really hitting the network
	if os.Getenv("USE_NETWORK") == "" {
		hd.Transport = &errorRoundTripper{"no such host"}
	}

	client := http.Client{Transport: hd}

	url := "http://bogus.bogus"
	client.Get(url)
	_, err := client.Get(url)
	if assert.NotNil(t, err) {
		assert.NotContains(t, err.Error(), "(cached)", "%s ForceErrors not honored", url)
	}
}

func TestHTTPDiskExpires(t *testing.T) {
	client := setupClient(t, Options{Expires: 100 * time.Millisecond})
	defer teardownClient(client)

	//
	// 1. miss
	//

	url := "http://httpbingo.org/get"
	resp, err := client.Get(url)
	if err != nil {
		t.Fatalf("Get %s failed %s", url, err)
	}
	defer resp.Body.Close()
	assert.Equal(t, "1", resp.Header.Get("X-Request-Id"))

	//
	// 2. hit
	//

	resp, err = client.Get(url)
	if err != nil {
		t.Fatalf("Get %s failed %s", url, err)
	}
	defer resp.Body.Close()
	assert.Equal(t, "1", resp.Header.Get("X-Request-Id"))

	//
	// 3. expired
	//

	time.Sleep(150 * time.Millisecond)

	resp, err = client.Get(url)
	if err != nil {
		t.Fatalf("Get %s failed %s", url, err)
	}
	defer resp.Body.Close()
	assert.Equal(t, "2", resp.Header.Get("X-Request-Id"))
}

func TestHTTPDiskStaleWhileRevalidate(t *testing.T) {
	var wg sync.WaitGroup

	client := setupClient(t, Options{Expires: 100 * time.Millisecond, StaleWhileRevalidate: true, RevalidationWaitGroup: &wg})
	defer teardownClient(client)

	//
	// 1. miss
	//

	url := "http://httpbingo.org/get"
	resp, err := client.Get(url)
	if err != nil {
		t.Fatalf("Get %s failed %s", url, err)
	}
	defer resp.Body.Close()
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, "1", resp.Header.Get("X-Request-Id"))

	//
	// 2. hit
	//

	resp, err = client.Get(url)
	if err != nil {
		t.Fatalf("Get %s failed %s", url, err)
	}
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, "1", resp.Header.Get("X-Request-Id"))

	//
	// 3. stale
	//

	time.Sleep(150 * time.Millisecond)

	resp, err = client.Get(url)
	if err != nil {
		t.Fatalf("Get %s failed %s", url, err)
	}
	defer resp.Body.Close()
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, "1", resp.Header.Get("X-Request-Id"))

	//
	// 4. refreshed in background
	//

	// Wait for background fetch to complete
	wg.Wait()

	resp, err = client.Get(url)
	if err != nil {
		t.Fatalf("Get %s failed %s", url, err)
	}
	defer resp.Body.Close()
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, "2", resp.Header.Get("X-Request-Id"))

	//
	// 5. stale again
	//

	time.Sleep(150 * time.Millisecond)

	resp, err = client.Get(url)
	if err != nil {
		t.Fatalf("Get %s failed %s", url, err)
	}
	defer resp.Body.Close()
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, "2", resp.Header.Get("X-Request-Id"))

	//
	// 4. refreshed in background
	//

	// Wait for background fetch to complete. This request returns a 502 error.
	wg.Wait()

	resp, err = client.Get(url)
	if err != nil {
		t.Fatalf("Get %s failed %s", url, err)
	}
	defer resp.Body.Close()
	assert.Equal(t, 502, resp.StatusCode)
	assert.Equal(t, "3", resp.Header.Get("X-Request-Id"))
}

func TestHTTPDiskNoCacheRevalidationErrors(t *testing.T) {
	var wg sync.WaitGroup

	client := setupClient(t, Options{
		Expires:                   100 * time.Millisecond,
		StaleWhileRevalidate:      true,
		NoCacheRevalidationErrors: true,
		RevalidationWaitGroup:     &wg,
	})
	defer teardownClient(client)

	//
	// 1. miss
	//

	url := "http://httpbingo.org/get"
	resp, err := client.Get(url)
	if err != nil {
		t.Fatalf("Get %s failed %s", url, err)
	}
	defer resp.Body.Close()
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, "1", resp.Header.Get("X-Request-Id"))

	//
	// 2. stale
	//

	time.Sleep(150 * time.Millisecond)

	resp, err = client.Get(url)
	if err != nil {
		t.Fatalf("Get %s failed %s", url, err)
	}
	defer resp.Body.Close()
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, "1", resp.Header.Get("X-Request-Id"))

	//
	// 3. got error in background, dropped
	//

	// Wait for background fetch to complete
	wg.Wait()

	resp, err = client.Get(url)
	if err != nil {
		t.Fatalf("Get %s failed %s", url, err)
	}
	defer resp.Body.Close()
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, "1", resp.Header.Get("X-Request-Id"))

	//
	// 4. refreshed in background, success
	//

	// Wait for background fetch to complete
	wg.Wait()

	resp, err = client.Get(url)
	if err != nil {
		t.Fatalf("Get %s failed %s", url, err)
	}
	defer resp.Body.Close()
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, "3", resp.Header.Get("X-Request-Id"))
}

func TestHTTPDiskTouchBeforeRevalidate(t *testing.T) {
	var wg sync.WaitGroup

	client := setupClient(t, Options{
		Expires:                   100 * time.Millisecond,
		StaleWhileRevalidate:      true,
		NoCacheRevalidationErrors: true,
		TouchBeforeRevalidate:     true,
		RevalidationWaitGroup:     &wg,
	})
	defer teardownClient(client)

	//
	// 1. miss
	//

	url := "http://httpbingo.org/get"
	resp, err := client.Get(url)
	if err != nil {
		t.Fatalf("Get %s failed %s", url, err)
	}
	defer resp.Body.Close()
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, "1", resp.Header.Get("X-Request-Id"))

	//
	// 2. stale
	//

	time.Sleep(150 * time.Millisecond)

	resp, err = client.Get(url)
	if err != nil {
		t.Fatalf("Get %s failed %s", url, err)
	}
	defer resp.Body.Close()
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, "1", resp.Header.Get("X-Request-Id"))

	//
	// 3. got error in background, dropped
	//

	// Wait for background fetch to complete
	wg.Wait()

	resp, err = client.Get(url)
	if err != nil {
		t.Fatalf("Get %s failed %s", url, err)
	}
	defer resp.Body.Close()
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, "1", resp.Header.Get("X-Request-Id"))

	//
	// 4. Not considered stale anymore, so still serving original response
	//

	// This should be a noop
	wg.Wait()

	resp, err = client.Get(url)
	if err != nil {
		t.Fatalf("Get %s failed %s", url, err)
	}
	defer resp.Body.Close()
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, "1", resp.Header.Get("X-Request-Id"))

	//
	// 5. stale again
	//

	time.Sleep(150 * time.Millisecond)

	resp, err = client.Get(url)
	if err != nil {
		t.Fatalf("Get %s failed %s", url, err)
	}
	defer resp.Body.Close()
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, "1", resp.Header.Get("X-Request-Id"))

	//
	// 5. refreshed in background, success
	//

	// Wait for background fetch to complete
	wg.Wait()

	resp, err = client.Get(url)
	if err != nil {
		t.Fatalf("Get %s failed %s", url, err)
	}
	defer resp.Body.Close()
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, "3", resp.Header.Get("X-Request-Id"))
}

func TestHTTPDiskStatus(t *testing.T) {
	hd := NewHTTPDisk(Options{Dir: TmpDir()})
	hd.Cache.RemoveAll()
	defer hd.Cache.RemoveAll()

	// 1. miss
	req := MustRequest("GET", "http://httpbingo.org/get")
	status, _ := hd.Status(req)
	assert.Equal(t, "miss", status.Status)

	// 2. hit
	MustWriteGzip(status.Path, "hello")
	status, _ = hd.Status(req)
	assert.Equal(t, "hit", status.Status)

	// 3. error
	MustWriteGzip(status.Path, "err:nope")
	status, _ = hd.Status(req)
	assert.Equal(t, "error", status.Status)
}

//
// Helpers
//

// Create and configure an http client for testing using httpdisk and recorder
// for the current test.
//
// Fixtures must be stored in fixtures/<testName>.yaml
//
// Callers are responsible for tearing everything down at the end of the test.
// For example:
//   client := setupClient(t, options)
//   defer teardownClient(client)
func setupClient(t *testing.T, hdOptions Options) *http.Client {
	// Default options
	if hdOptions.Dir == "" {
		hdOptions.Dir = TmpDir()
	}

	hd := NewHTTPDisk(hdOptions)
	hd.Cache.RemoveAll()

	vcr := newVCR(t)
	hd.Transport = vcr

	return &http.Client{Transport: hd}
}

func teardownClient(client *http.Client) {
	hd := client.Transport.(*HTTPDisk)
	hd.Cache.RemoveAll()

	vcr := hd.Transport.(*recorder.Recorder)
	vcr.Stop()
}

// Create a new recorder for the current test.
// Fixtures must be stored in fixtures/<testName>.yaml
//
// Callers are responsible for stopping the returned vcr. For example:
//   vcr := newVCR(t)
//   defer vcr.Stop()
func newVCR(t *testing.T) *recorder.Recorder {
	cassette := fmt.Sprintf("fixtures/%s", t.Name())
	vcr, err := recorder.New(cassette)
	if err != nil {
		t.Fatal(err)
	}

	return vcr
}

//
// Custom RoundTripper that always returns an error
//

type errorRoundTripper struct{ errorString string }

func (t *errorRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	return nil, errors.New(t.errorString)
}
