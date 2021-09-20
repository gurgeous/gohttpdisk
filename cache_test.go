package gohttpdisk

import (
	"testing"
)

func TestCacheGet(t *testing.T) {
	c := newCache(Options{Dir: TmpDir()})
	c.RemoveAll()
	defer c.RemoveAll()

	//
	// get (not found)
	//

	ck := MustCacheKey(MustRequest("GET", "http://a.com/b"))
	data, err := c.Get(ck)
	if len(data) != 0 {
		t.Fatal("Get - data should be empty")
	}
	if err == nil {
		t.Fatal("Get - should have failed")
	}

	// set
	err = c.Set(ck, []byte("hello"))
	if err != nil {
		t.Fatalf("Set - failed with %s", err)
	}

	// now get should work
	data, err = c.Get(ck)
	if err != nil {
		t.Fatal("Get - should not have failed")
	}
	if string(data) != "hello" {
		t.Fatalf("Get - expected %s but got %s", "hello", string(data))
	}
}
