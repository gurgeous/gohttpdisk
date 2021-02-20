package httpdisk

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
)

func TestCacheGet(t *testing.T) {
	c := newCache(Options{})
	c.RemoveAll()
	defer c.RemoveAll()

	//
	// get (not found)
	//

	req := req("GET", "http://a.com/b")
	data, err := c.Get(req)
	if len(data) != 0 {
		t.Fatal("Get - data should be empty")
	}
	if err == nil {
		t.Fatal("Get - should have failed")
	}

	// set
	err = c.Set(req, []byte("hello"))
	if err != nil {
		t.Fatalf("Set - failed with %s", err)
	}

	// now get should work
	data, err = c.Get(req)
	if string(data) != "hello" {
		t.Fatalf("Get - expected %s but got %s", "hello", string(data))
	}
}

// key normalization
func TestCacheKeys(t *testing.T) {
	c := newCache(Options{})

	assertMatch := func(a *http.Request, b *http.Request) {
		c1 := c.Canonical(a)
		c2 := c.Canonical(b)
		if c1 != c2 {
			t.Fatalf("cache keys don't match %s != %s", c1, c2)
		}
	}
	assertDiffer := func(a *http.Request, b *http.Request) {
		c1 := c.Canonical(a)
		c2 := c.Canonical(b)
		if c1 == c2 {
			t.Fatalf("cache keys don't differ %s == %s", c1, c2)
		}
	}

	// these pairs should match
	match := [][]string{
		{"http://a.com?a=1&a=2&b=2&c=3", "HTTP://A.COM:80?c=3&b=2&a=2&a=1"},
		{"https://a.com?a=1&b=2&c=3", "HTTPs://A.COM:443?c=3&b=2&a=1"},
		{"https://a.com?", "HTTPs://A.COM:443/"},
	}
	for _, pair := range match {
		assertMatch(req("GET", pair[0]), req("GET", pair[1]))
	}

	// methods differ, keys differ
	assertDiffer(req("GET", "http://a.com"), req("HEAD", "http://a.com"))

	// bodies differ, keys differ
	req1, _ := http.NewRequest("POST", "http://a.com", strings.NewReader("abc"))
	req2, _ := http.NewRequest("POST", "http://a.com", strings.NewReader("def"))
	assertDiffer(req1, req2)
}

func TestCacheHost(t *testing.T) {
	hostDir := fmt.Sprintf("%ca.com%c", os.PathSeparator, os.PathSeparator)

	// w/o NoHosts
	urls := []string{"http://a.com", "http://www.a~~.com"}
	for _, url := range urls {
		path := newCache(Options{}).Path(req("GET", url))
		if !strings.Contains(path, hostDir) {
			t.Fatalf("path %s for url %s should contain %s", path, url, hostDir)
		}
	}

	// w/ NoHosts
	path1 := newCache(Options{NoHosts: true}).Path(req("GET", "http://a.com"))
	if strings.Contains(path1, hostDir) {
		t.Fatalf("path %s shouldn't contain %s", path1, hostDir)
	}
}

// create a test request
func req(method string, url string) *http.Request {
	r, _ := http.NewRequest(method, url, nil)
	return r
}
