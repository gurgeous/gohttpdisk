package gohttpdisk

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httputil"
	"strings"
	"sync"
	"time"
)

// HTTPDisk is a caching http transport.
type HTTPDisk struct {
	// Underlying Cache.
	Cache Cache
	// if nil, http.DefaultTransport is used.
	Transport http.RoundTripper
	Options   Options
}

// Options for creating a new HTTPDisk.
type Options struct {
	// Directory where the cache is stored. Defaults to httpdisk.
	Dir string

	// When to expire cached requests. Less than or equal to zero disables.
	Expires time.Duration

	// Don't read anything from cache (but still write)
	Force bool

	// Don't read errors from cache (but still write)
	ForceErrors bool

	// Optional logger
	Logger *log.Logger

	// Don't cache errors during revalidation. Leave stale data in cache instead.
	NoCacheRevalidationErrors bool

	// If StaleWhileRevalidate is enabled, you may optionally set this wait group
	// to be notified when background fetches complete.
	RevalidationWaitGroup *sync.WaitGroup

	// Return stale cached responses while refreshing the cache in the background.
	// Only relevant if expires is set.
	StaleWhileRevalidate bool

	// Update cache file modification time before kicking off a background revalidation.
	// Helps guard against thundering herd problem, but risks leaving stale data in the
	// cache longer than expected.
	TouchBeforeRevalidate bool
}

type Status struct {
	Digest string
	Key    string
	Path   string
	Status string
	URL    string
}

type Entry struct {
	Response *http.Response
	Age      time.Duration
}

const errPrefix = "err:"

// NewHTTPDisk constructs a new HTTPDisk.
func NewHTTPDisk(options Options) *HTTPDisk {
	return &HTTPDisk{Cache: *newCache(options), Options: options}
}

func (hd *HTTPDisk) Status(req *http.Request) (*Status, error) {
	cacheKey, err := NewCacheKey(req)
	if err != nil {
		return nil, err
	}

	// what is the status?
	data, _, _ := hd.Cache.Get(cacheKey)
	var status string
	if len(data) == 0 {
		status = "miss"
	} else if bytes.HasPrefix(data, []byte(errPrefix)) {
		status = "error"
	} else {
		status = "hit"
	}

	return &Status{
		Digest: cacheKey.Digest(),
		Key:    cacheKey.Key(),
		Path:   hd.Cache.diskpath(cacheKey),
		Status: status,
		URL:    req.URL.String(),
	}, nil
}

// RoundTrip is the entry point used by http.Client.
func (hd *HTTPDisk) RoundTrip(req *http.Request) (resp *http.Response, err error) {
	entry, err := hd.performRoundTrip(req)
	if err != nil {
		return
	}

	resp = entry.Response

	// Handle stale responses
	if hd.Options.Expires > 0 && hd.Options.Expires < entry.Age {
		if hd.Options.StaleWhileRevalidate {
			// Revalidate in the background while returning stale data.
			hd.backgroundRevalidate(req)
		} else {
			// Must fetch and return fresh data.
			resp, err = hd.fetch(req, true)
		}
	}

	return
}

func (hd *HTTPDisk) performRoundTrip(req *http.Request) (entry *Entry, err error) {
	cacheKey, err := NewCacheKey(req)
	if err != nil {
		return nil, err
	}

	// Get our cached response unless Force is on, in which case we ignore cached data
	if !hd.Options.Force {
		entry, err = hd.get(cacheKey)

		// Handle the following possible cases for cached data:
		//  1. Network error: use cache if ForceErrors=false, otherwise hit network
		//  2. HTTP 400 or 500: use cache if ForceErrors=false, otherwise hit network
		//  3. HTTP 200 or 300: use cache
		//  4. Nothing in cache: hit network

		if err != nil {
			if !hd.Options.ForceErrors {
				// Return cached network error
				return nil, err
			}
		} else if entry != nil {
			if !isHttpError(entry.Response) || !hd.Options.ForceErrors {
				// Return cached response
				return entry, nil
			}
		}
	}

	// not found. make the request
	resp, err := hd.fetch(req, true)
	if err != nil {
		return nil, err
	}
	return &Entry{Response: resp}, nil
}

func (hd *HTTPDisk) fetch(req *http.Request, cacheErrors bool) (resp *http.Response, err error) {
	transport := hd.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}

	cacheKey, err := NewCacheKey(req)
	if err != nil {
		return nil, err
	}

	// not found. make the request
	if hd.Options.Logger != nil {
		hd.Options.Logger.Printf("%s %s", req.Method, req.URL)
	}

	start := time.Now()
	resp, err = transport.RoundTrip(req)
	if err != nil {
		if hd.Options.Logger != nil {
			hd.Options.Logger.Printf("Network error on %s (%s)", req.URL, err)
		}

		if cacheErrors {
			err = hd.handleError(cacheKey, err)
		}
		return nil, err
	}

	if hd.Options.Logger != nil && isHttpError(resp) {
		hd.Options.Logger.Printf("Http error on %s (%s)", req.URL, resp.Status)
	}

	// cache response
	err = hd.set(cacheKey, resp, start, cacheErrors)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

// Launch goroutine to refresh the cache
func (hd *HTTPDisk) backgroundRevalidate(req *http.Request) {
	// Update timestamp on old file before proceeding. Protection against
	// thundering herd.
	if hd.Options.TouchBeforeRevalidate {
		cacheKey, err := NewCacheKey(req)
		if err == nil {
			hd.Cache.Touch(cacheKey)
		}
	}

	if hd.Options.RevalidationWaitGroup != nil {
		hd.Options.RevalidationWaitGroup.Add(1)
	}

	// Clone the request so that we can reissue it without being tied
	// to the current request context. Otherwise we risk being cancelled
	// when the main thread returns.
	req = req.Clone(context.Background())

	go func() {
		if hd.Options.RevalidationWaitGroup != nil {
			defer hd.Options.RevalidationWaitGroup.Done()
		}
		hd.fetch(req, !hd.Options.NoCacheRevalidationErrors)
	}()
}

// get cached response for this request, if any
func (hd *HTTPDisk) get(cacheKey *CacheKey) (*Entry, error) {
	data, age, err := hd.Cache.Get(cacheKey)
	if len(data) == 0 {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	// is it a cached error?
	if bytes.HasPrefix(data, []byte(errPrefix)) {
		errString := string(data[len(errPrefix):])
		return nil, fmt.Errorf("%s (cached)", errString)
	}

	buf := bytes.NewBuffer(data)
	resp, err := http.ReadResponse(bufio.NewReader(buf), cacheKey.Request)
	if err != nil {
		return nil, err
	}
	return &Entry{Response: resp, Age: age}, nil
}

// set cached response
func (hd *HTTPDisk) set(cacheKey *CacheKey, resp *http.Response, start time.Time, cacheErrors bool) error {
	// drain body, put back into Response
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		// errors can occur here if the server returns an invalid body. handle that
		// case and consider caching the error
		if cacheErrors {
			err = hd.handleError(cacheKey, err)
		}
		return err
	}
	resp.Body = ioutil.NopCloser(bytes.NewBuffer(body))

	// short circuit for http errors
	if !cacheErrors && isHttpError(resp) {
		return nil
	}

	// add our headers
	elapsed := float64(time.Since(start)) / float64(time.Second)
	resp.Header.Set("X-Gohttpdisk-Elapsed", fmt.Sprintf("%0.3f", elapsed))
	resp.Header.Set("X-Gohttpdisk-Url", cacheKey.Request.URL.String())

	// now cache bytes
	data, err := httputil.DumpResponse(resp, true)
	if err != nil {
		return err
	}
	err = hd.Cache.Set(cacheKey, data)
	if err != nil {
		return err
	}

	// restore body
	resp.Body = ioutil.NopCloser(bytes.NewBuffer(body))
	return nil
}

func (hd *HTTPDisk) handleError(cacheKey *CacheKey, err error) error {
	if isCacheableError(err) {
		err2 := hd.setError(cacheKey, err)
		if err2 != nil {
			// error while caching, give the caller a chance to see it
			err = err2
		}
	}
	return err
}

// cache an error response
func (hd *HTTPDisk) setError(cacheKey *CacheKey, err error) error {
	body := fmt.Sprintf("%s%s", errPrefix, err.Error())
	err2 := hd.Cache.Set(cacheKey, []byte(body))
	if err2 != nil {
		return err2
	}
	return nil
}

// if err.Error() contains one of these, we consider the error to be cacheable
// and we write it to disk. This list was generated by hitting the tranco top
// 1000 websites.
var cacheableErrors = []string{
	"certificate has expired",
	"certificate is valid",
	"certificate signed by unknown authority",
	"connection refused",
	"connection reset by peer",
	"context deadline exceeded",
	"EOF",
	"handshake failure",
	"i/o timeout",
	"no route to host",
	"no such host",
	"request canceled",
	"stream error",
	"tls: internal error",
	"tls: unrecognized name",
}

func isCacheableError(err error) bool {
	errorString := err.Error()
	for _, s := range cacheableErrors {
		if strings.Contains(errorString, s) {
			return true
		}
	}

	fmt.Printf("isCacheableError? type:%T v:%v\n", err, err)
	return false
}

func isHttpError(resp *http.Response) bool {
	return resp.StatusCode >= 400
}
