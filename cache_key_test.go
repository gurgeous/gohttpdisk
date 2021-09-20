package gohttpdisk

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// key normalization
func TestCacheKeys(t *testing.T) {
	assertMatch := func(a *http.Request, b *http.Request) {
		c1, c2 := MustCacheKey(a), MustCacheKey(b)
		assert.Equal(t, c1.Key(), c2.Key())
	}
	assertDiffer := func(a *http.Request, b *http.Request) {
		c1, c2 := MustCacheKey(a), MustCacheKey(b)
		assert.NotEqual(t, c1.Key(), c2.Key())
	}

	// these pairs should match
	match := [][]string{
		{"http://a.com?a=1&a=2&b=2&c=3", "HTTP://A.COM:80?c=3&b=2&a=2&a=1"},
		{"https://a.com?a=1&b=2&c=3", "HTTPs://A.COM:443?c=3&b=2&a=1"},
		{"https://a.com?", "HTTPs://A.COM:443/"},
	}
	for _, pair := range match {
		assertMatch(MustRequest("GET", pair[0]), MustRequest("GET", pair[1]))
	}

	// methods differ, keys differ
	assertDiffer(MustRequest("GET", "http://a.com"), MustRequest("HEAD", "http://a.com"))

	// bodies differ, keys differ
	req1, _ := http.NewRequest("POST", "http://a.com", strings.NewReader("abc"))
	req2, _ := http.NewRequest("POST", "http://a.com", strings.NewReader("def"))
	assertDiffer(req1, req2)
}

func TestCacheHost(t *testing.T) {
	hostDir := fmt.Sprintf("a.com%c", os.PathSeparator)

	urls := []string{"http://a.com", "http://www.a~~.com"}
	for _, url := range urls {
		ck := MustCacheKey(MustRequest("GET", url))
		path := ck.Diskpath()
		if !strings.HasPrefix(path, hostDir) {
			t.Fatalf("path %s for url %s should start with %s", path, url, hostDir)
		}
	}
}
