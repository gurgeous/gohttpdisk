package gohttpdisk

import (
	"compress/gzip"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
)

// create cache key
func MustCacheKey(req *http.Request) *CacheKey {
	ck, err := NewCacheKey(req)
	if err != nil {
		panic(err)
	}
	return ck
}

// create a test request
func MustRequest(method string, url string) *http.Request {
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		panic(err)
	}
	return req
}

// parse url
func MustURL(s string) *url.URL {
	u, err := url.Parse(s)
	if err != nil {
		panic(err)
	}
	return u
}

// write data to path
func MustWriteGzip(path string, data string) {
	err := os.MkdirAll(filepath.Dir(path), os.ModePerm)
	if err != nil {
		panic(err)
	}
	f, err := os.Create(path)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	gz := gzip.NewWriter(f)
	defer gz.Close()
	gz.Write([]byte(data))
}

// a temp dir where we can place our cache
func TmpDir() string {
	dir, err := ioutil.TempDir("", "gohttpdisk")
	if err != nil {
		panic(err)
	}
	return dir
}
