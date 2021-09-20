package gohttpdisk

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// a key in the cache
type CacheKey struct {
	Request *http.Request
}

func NewCacheKey(req *http.Request) (*CacheKey, error) {
	if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
		return nil, fmt.Errorf("http/https required (%s)", req.URL.String())
	}
	if req.URL.Host == "" {
		return nil, fmt.Errorf("host required (%s)", req.URL.String())
	}
	return &CacheKey{req}, nil
}

// Key calculates a canonical cache key for the request based on the http
// method, the normalized URL, and the request body if present. The key can be
// quite long since it contains the request body.
func (cacheKey *CacheKey) Key() string {
	method := strings.ToUpper(cacheKey.Request.Method)
	if method == "" {
		method = "GET"
	}

	scheme := strings.ToLower(cacheKey.Request.URL.Scheme)
	port := cacheKey.Request.URL.Port()
	if port == "" {
		port = ports[scheme]
	}

	path := cacheKey.Request.URL.Path
	if path == "" {
		path = "/"
	}

	key := make([]string, 12)
	key = append(key, method)
	key = append(key, " ")
	key = append(key, scheme)
	key = append(key, "://")
	key = append(key, strings.ToLower(cacheKey.Request.URL.Hostname()))
	if port != ports[scheme] {
		key = append(key, ":")
		key = append(key, port)
	}
	if path != "/" {
		key = append(key, path)
	}
	if query := cacheKey.Request.URL.Query(); len(query) > 0 {
		key = append(key, "?")
		key = append(key, querykey(query))
	}
	if cacheKey.Request.GetBody != nil {
		key = append(key, " ")
		key = append(key, bodykey(cacheKey.Request))
	}

	return strings.Join(key, "")
}

// Digest returns the md5 sum for this request.
func (cacheKey *CacheKey) Digest() string {
	return md5String(cacheKey.Key())
}

// Path returns the path on disk for this request.
func (cacheKey *CacheKey) Diskpath() string {
	paths := []string{}

	// Dir
	paths = append(paths, normalizeHostForPath(cacheKey.Request.URL.Hostname()))

	// Key
	key := cacheKey.Digest()
	paths = append(paths, key[0:2])
	paths = append(paths, key[2:4])
	paths = append(paths, key[4:])

	return filepath.Join(paths...)
}

//
// helpers
//

var ports = map[string]string{
	"http":  "80",
	"https": "443",
}

func querykey(query url.Values) string {
	for key := range query {
		sort.Strings(query[key])
	}
	return query.Encode() // note: sorts by key
}

func bodykey(req *http.Request) string {
	reader, err := req.GetBody()
	if err != nil {
		return ""
	}
	defer reader.Close()
	data, err := ioutil.ReadAll(reader)
	if err != nil {
		return ""
	}
	return string(data)
}

var (
	hostWwwRe   = regexp.MustCompile(`^(www\.)`)
	hostCharsRe = regexp.MustCompile("[^A-Za-z0-9._-]+")
)

// Normalize a hostname. Collisions are ok because the rest of the path is an
// md5 checksum.
func normalizeHostForPath(s string) string {
	s = hostWwwRe.ReplaceAllString(s, "")
	s = hostCharsRe.ReplaceAllString(s, "")
	return s
}

func md5String(text string) string {
	hash := md5.Sum([]byte(text))
	return hex.EncodeToString(hash[:])
}
