package httpdisk

import (
	"bufio"
	"bytes"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httputil"
	"regexp"
	"strings"
)

// HTTPDisk is a caching http transport.
type HTTPDisk struct {
	// Underlying Cache.
	Cache Cache
	// if nil, http.DefaultTransport is used.
	Transport http.RoundTripper
}

// Options for creating a new HTTPDisk.
type Options struct {
	// Directory where the cache is stored. Defaults to httpdisk.
	Dir string
	// If true, include the request hostname in the path for each element.
	NoHosts bool
}

const errPrefix = "err:"

// NewHTTPDisk constructs a new HTTPDisk.
func NewHTTPDisk(options Options) *HTTPDisk {
	return &HTTPDisk{Cache: *newCache(options)}
}

// RoundTrip is the entry point used by http.Client.
func (hd *HTTPDisk) RoundTrip(req *http.Request) (*http.Response, error) {
	transport := hd.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}

	// get our cached response
	resp, err := hd.get(req)
	if err != nil {
		return nil, err
	}
	if resp != nil {
		return resp, nil
	}

	// not found. make the request
	resp, err = transport.RoundTrip(req)
	if err != nil {
		if isCacheableError(err) {
			err = hd.setError(req, err)
			return nil, err
		}
		return nil, err
	}

	// cache response
	err = hd.set(req, resp)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

// get cached response for this request, if any
func (hd *HTTPDisk) get(req *http.Request) (*http.Response, error) {
	data, err := hd.Cache.Get(req)
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
	return http.ReadResponse(bufio.NewReader(buf), req)
}

// set cached response
func (hd *HTTPDisk) set(req *http.Request, resp *http.Response) error {
	// drain body
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	// cache (w/ body)
	resp.Body = ioutil.NopCloser(bytes.NewBuffer(body))
	data, err := httputil.DumpResponse(resp, true)
	if err != nil {
		return err
	}
	err = hd.Cache.Set(req, data)
	if err != nil {
		return err
	}

	// restore body
	resp.Body = ioutil.NopCloser(bytes.NewBuffer(body))
	return nil
}

// cache an error response
func (hd *HTTPDisk) setError(req *http.Request, err error) error {
	body := fmt.Sprintf("%s%s", errPrefix, err.Error())
	err2 := hd.Cache.Set(req, []byte(body))
	if err2 != nil {
		return err2
	}
	return nil
}

var (
	cacheableErrors = []string{
		"certificate is valid",
		"context deadline exceeded",
		"EOF",
		"request canceled",
		"stream error",
	}
	cacheableRegex = regexp.MustCompile(strings.Join(cacheableErrors, "|"))
)

func isCacheableError(err error) bool {
	switch err.(type) {
	case *net.OpError:
		return true
	}
	if cacheableRegex.MatchString(err.Error()) {
		return true
	}

	fmt.Printf("isCacheableError? type:%T v:%v\n", err, err)
	return false
}
