package gohttpdisk

import (
	"compress/gzip"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"
)

// Cache will cache http.Responses on disk, using the http.Request to calculate
// a key. It deals with keys and files, not the network.
type Cache struct {
	// Directory where the cache is stored. Defaults to gohttpdisk.
	Dir string

	// If true, don't include the request hostname in the path for each element.
	NoHosts bool
}

func newCache(options Options) *Cache {
	if options.Dir == "" {
		options.Dir = "gohttpdisk"
	}
	return &Cache{Dir: options.Dir, NoHosts: options.NoHosts}
}

// Get the cached data for a request. An empty byte array will be returned if
// the entry doesn't exist or can't be read for any reason.
func (cache *Cache) Get(cacheKey *CacheKey) (data []byte, age time.Duration, err error) {
	path := cache.diskpath(cacheKey)
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return
	}
	defer gz.Close()

	age = cache.age(path)
	data, err = ioutil.ReadAll(gz)

	return
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

// Update the modified time if the cached file exists.
func (cache *Cache) Touch(cacheKey *CacheKey) error {
	path := cache.diskpath(cacheKey)
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		// Do nothing if the file doesn't exist
		return nil
	}

	currentTime := time.Now()
	return os.Chtimes(path, currentTime, currentTime)
}

// RemoveAll unlinks the cache.
func (cache *Cache) RemoveAll() error {
	return os.RemoveAll(cache.Dir)
}

//
// helpers
//

func (cache *Cache) diskpath(cacheKey *CacheKey) string {
	return filepath.Join(cache.Dir, cacheKey.Diskpath(cache.NoHosts))
}

func (cache *Cache) age(path string) time.Duration {
	stat, err := os.Stat(path)
	if err != nil {
		return 0
	}

	return time.Since(stat.ModTime())
}
