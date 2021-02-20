package httpdisk

import (
	"compress/gzip"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Cache will cache http.Responses on disk, using the http.Request to calculate
// a key. It deals with keys and files, not the network.
type Cache struct {
	// Directory where the cache is stored. Defaults to httpdisk.
	Dir string
	// If true, don't include the request hostname in the path for each element.
	NoHosts bool
}

func newCache(options Options) *Cache {
	if options.Dir == "" {
		options.Dir = "httpdisk"
	}
	return &Cache{options.Dir, options.NoHosts}
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

	// NoHosts: true
	if !c.NoHosts {
		paths = append(paths, normalizeHostForPath(req.URL.Hostname()))
	}

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
	scheme := strings.ToLower(req.URL.Scheme)
	port := req.URL.Port()
	if port == "" {
		port = ports[scheme]
	}
	path := req.URL.Path
	if path == "" {
		path = "/"
	}

	parts := make([]string, 10)
	parts = append(parts, req.Method)
	parts = append(parts, " ")
	parts = append(parts, scheme)
	parts = append(parts, "://")
	parts = append(parts, strings.ToLower(req.URL.Hostname()))
	parts = append(parts, ":")
	parts = append(parts, port)
	parts = append(parts, path)

	// sort query
	if query := req.URL.Query(); len(query) > 0 {
		parts = append(parts, "?")
		for key := range query {
			sort.Strings(query[key])
		}
		parts = append(parts, query.Encode()) // note: sorts by key
	}

	// add body
	if req.GetBody != nil {
		reader, err := req.GetBody()
		if err == nil {
			defer reader.Close()
			data, err := ioutil.ReadAll(reader)
			if err == nil {
				parts = append(parts, string(data))
			}
		}
	}

	return strings.Join(parts, "")
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
