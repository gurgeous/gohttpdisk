package gohttpdisk

import (
	"io/ioutil"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestHTTPDisk(t *testing.T) {
	hd := NewHTTPDisk(Options{Dir: TmpDir()})
	hd.Cache.RemoveAll()
	defer hd.Cache.RemoveAll()

	client := http.Client{Transport: hd}

	drainBody := func(resp *http.Response) string {
		data, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			panic(err)
		}
		return string(data)
	}

	//
	// 1. miss
	//

	url := "http://httpbingo.org/get"
	resp, err := client.Get(url)
	if err != nil {
		t.Fatalf("Get %s failed %s", url, err)
	}
	defer resp.Body.Close()
	body1 := drainBody(resp)
	date1 := resp.Header.Get("Date")

	//
	// 2. hit
	//

	resp, err = client.Get(url)
	if err != nil {
		t.Fatalf("Get %s failed %s", url, err)
	}
	defer resp.Body.Close()
	assert.Equal(t, body1, drainBody(resp))
	assert.Equal(t, date1, resp.Header.Get("Date"))
}

func TestHTTPDiskErrors(t *testing.T) {
	hd := NewHTTPDisk(Options{Dir: TmpDir()})
	hd.Cache.RemoveAll()
	defer hd.Cache.RemoveAll()

	var err error

	client := http.Client{Transport: hd, Timeout: time.Millisecond * 500}

	// bad host
	url := "http://bogus.bogus"
	client.Get(url)
	_, err = client.Get(url)
	if !strings.Contains(err.Error(), "(cached)") {
		t.Fatalf("%s error was not cached", url)
	}

	// timeout
	url = "http://httpbingo.org/delay/1"
	client.Get(url)
	_, err = client.Get(url)
	if !strings.Contains(err.Error(), "(cached)") {
		t.Fatalf("%s error was not cached", url)
	}
}

func TestHTTPDiskStatus(t *testing.T) {
	hd := NewHTTPDisk(Options{Dir: TmpDir()})
	hd.Cache.RemoveAll()
	defer hd.Cache.RemoveAll()

	// 1. miss
	req := MustRequest("GET", "http://httpbingo.org/get")
	status, _ := hd.Status(req)
	assert.Equal(t, "miss", status.Status)

	// 2. hit
	MustWriteGzip(status.Path, "hello")
	status, _ = hd.Status(req)
	assert.Equal(t, "hit", status.Status)

	// 3. error
	MustWriteGzip(status.Path, "err:nope")
	status, _ = hd.Status(req)
	assert.Equal(t, "error", status.Status)
}
