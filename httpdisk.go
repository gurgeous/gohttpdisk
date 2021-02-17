package httpdisk

import (
	"bufio"
	"bytes"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
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
	HostInPath bool
}

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

	buf := bytes.NewBuffer(data)
	return http.ReadResponse(bufio.NewReader(buf), req)
}

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
