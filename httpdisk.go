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
	Cache *Cache
	// if nil, http.DefaultTransport is used.
	Transport http.RoundTripper
}

// NewHTTPDisk creates a new HTTPDisk. Responses are written to dir. If host is
// true, the hostname is used in the path as well.
func NewHTTPDisk(dir string, host bool) *HTTPDisk {
	c := NewCache(dir, host)
	return &HTTPDisk{Cache: c}
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
