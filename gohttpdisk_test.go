package gohttpdisk

import (
	"io/ioutil"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestHTTPDisk(t *testing.T) {
	hd := NewHTTPDisk(Options{})
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
	// 1. first get (not cached)
	//

	url := "http://httpbin.org/anything"
	resp, err := client.Get(url)
	if err != nil {
		t.Fatalf("Get %s failed %s", url, err)
	}
	defer resp.Body.Close()
	body1 := drainBody(resp)
	date1 := resp.Header.Get("Date")

	//
	// 2. second get (cached)
	//

	resp, err = client.Get(url)
	if err != nil {
		t.Fatalf("Get %s failed %s", url, err)
	}
	defer resp.Body.Close()
	body2 := drainBody(resp)
	date2 := resp.Header.Get("Date")

	if body1 != body2 {
		t.Fatalf("Second GET had different body %s != %s", body1, body2)
	}
	if date1 != date2 {
		t.Fatalf("Second GET had different date %s != %s", date1, date2)
	}
}

func TestHTTPDiskErrors(t *testing.T) {
	hd := NewHTTPDisk(Options{})
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
	url = "http://httpbin.org/delay/1"
	client.Get(url)
	_, err = client.Get(url)
	if !strings.Contains(err.Error(), "(cached)") {
		t.Fatalf("%s error was not cached", url)
	}
}
