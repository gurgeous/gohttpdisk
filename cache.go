package gohttpdisk

import (
	"compress/gzip"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
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
func (cache *Cache) Get(cacheKey *CacheKey) ([]byte, error) {
	f, err := os.Open(cache.diskpath(cacheKey))
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
func (cache *Cache) Set(cacheKey *CacheKey, data []byte) error {
	// make sure directory exists
	diskpath := cache.diskpath(cacheKey)
	if err := os.MkdirAll(filepath.Dir(diskpath), os.ModePerm); err != nil {
		return err
	}

	// write to tmp file in same directory
	tmp := filepath.Join(filepath.Dir(diskpath), fmt.Sprintf(".tmp-%s", filepath.Base(diskpath)))
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
	if err := os.Rename(tmp, diskpath); err != nil {
		return err
	}

	return nil
}

// RemoveAll unlinks the cache.
func (cache *Cache) RemoveAll() error {
	return os.RemoveAll(cache.Dir)
}

//
// helpers
//

func (cache *Cache) diskpath(cacheKey *CacheKey) string {
	return filepath.Join(cache.Dir, cacheKey.Diskpath())
}
