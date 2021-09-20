package gohttpdisk

import (
	"compress/gzip"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Cache will cache http.Responses on disk, using the http.Request to calculate
// a key. It deals with keys and files, not the network.
type Cache struct {
	// Directory where the cache is stored. Defaults to gohttpdisk.
	Dir string
}

func newCache(options Options) *Cache {
	if options.Dir == "" {
		options.Dir = "gohttpdisk"
	}
	return &Cache{options.Dir}
}

// Get the cached data for a request. An empty byte array will be returned if
// the entry doesn't exist or can't be read for any reason.
func (c *Cache) Get(req *http.Request) ([]byte, error) {
	f, err := os.Open(c.Path(req))
	if err != nil {
		return nil, err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	defer gz.Close()

	return ioutil.ReadAll(gz)
}

// Set cached data for a request.
func (c *Cache) Set(req *http.Request, data []byte) error {
	path := c.Path(req)

	// make sure directory exists
	if err := os.MkdirAll(filepath.Dir(path), os.ModePerm); err != nil {
		return err
	}

	// write to tmp file in same directory
	tmp := filepath.Join(filepath.Dir(path), fmt.Sprintf(".tmp-%s", filepath.Base(path)))
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	defer os.Remove(tmp)

	// write compressed data
	gz := gzip.NewWriter(f)
	if _, err := gz.Write(data); err != nil {
		return err
	}
	if err := gz.Close(); err != nil {
		return err
	}

	if err := f.Close(); err != nil {
		return err
	}

	// move into place
	if err := os.Rename(tmp, path); err != nil {
		return err
	}

	return nil
}

// Key returns the md5 sum for this request.
func (c *Cache) Key(req *http.Request) string {
	return md5String(c.Canonical(req))
}

// Path returns the full path on disk for this request.
func (c *Cache) Path(req *http.Request) string {
	paths := []string{}

	// Dir
	paths = append(paths, c.Dir)
	paths = append(paths, normalizeHostForPath(req.URL.Hostname()))

	// Key
	key := c.Key(req)
	paths = append(paths, key[0:2])
	paths = append(paths, key[2:4])
	paths = append(paths, key[4:])

	return filepath.Join(paths...)
}

// RemoveAll unlinks the cache.
func (c *Cache) RemoveAll() error {
	return os.RemoveAll(c.Dir)
}

var ports = map[string]string{
	"http":  "80",
	"https": "443",
}

// Canonical calculates a signature for the request based on the http method,
// the normalized URL, and the request body if present. The signature can be
// quite long since it contains the request body.
func (c *Cache) Canonical(req *http.Request) string {
	method := strings.ToUpper(req.Method)
	if method == "" {
		method = "GET"
	}

	scheme := strings.ToLower(req.URL.Scheme)
	port := req.URL.Port()
	if port == "" {
		port = ports[scheme]
	}

	path := req.URL.Path
	if path == "" {
		path = "/"
	}

	key := make([]string, 12)
	key = append(key, method)
	key = append(key, " ")
	key = append(key, scheme)
	key = append(key, "://")
	key = append(key, strings.ToLower(req.URL.Hostname()))
	if port != ports[scheme] {
		key = append(key, ":")
		key = append(key, port)
	}
	if path != "/" {
		key = append(key, path)
	}
	if query := req.URL.Query(); len(query) > 0 {
		key = append(key, "?")
		key = append(key, c.Querykey(query))
	}
	if req.GetBody != nil {
		key = append(key, " ")
		key = append(key, c.Bodykey(req))
	}

	return strings.Join(key, "")
}

func (c *Cache) Querykey(query url.Values) string {
	for key := range query {
		sort.Strings(query[key])
	}
	return query.Encode() // note: sorts by key
}

func (c *Cache) Bodykey(req *http.Request) string {
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
