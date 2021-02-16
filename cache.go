package httpdisk

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Cache will cache http.Responses on disk, using the http.Request to calculate
// a key. It mainly deals with keys and files, not the network.
type Cache struct {
	Dir string
}

// NewCache creates a new Cache. Files will be stored in dir.
func NewCache(dir string) *Cache {
	return &Cache{dir}
}

// Get the cached data for a request. An empty byte array will be returned if
// the entry doesn't exist or can't be read for any reason.
func (c *Cache) Get(req *http.Request) ([]byte, error) {
	return ioutil.ReadFile(c.RequestPath(req))
}

// Set cached data for a request.
func (c *Cache) Set(req *http.Request, data []byte) error {
	path := c.RequestPath(req)

	// make sure directory exists
	if err := os.MkdirAll(filepath.Dir(path), os.ModePerm); err != nil {
		return err
	}

	// write to tmp file
	tmp := fmt.Sprintf("%s/.tmp-%s", filepath.Dir(path), filepath.Base(path))
	if err := ioutil.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	defer os.Remove(tmp)

	// move into place
	if err := os.Rename(tmp, path); err != nil {
		return err
	}

	return nil
}

// RequestKey returns the md5 sum for this request.
func (c *Cache) RequestKey(req *http.Request) string {
	return md5String(c.Canonical(req))
}

// RequestPath returns the full path on disk for this request.
func (c *Cache) RequestPath(req *http.Request) string {
	key := c.RequestKey(req)
	return filepath.Join(c.Dir, key[0:2], key[2:4], key[4:])
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

func md5String(text string) string {
	hash := md5.Sum([]byte(text))
	return hex.EncodeToString(hash[:])
}
