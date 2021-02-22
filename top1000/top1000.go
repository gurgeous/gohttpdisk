package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/cookiejar"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gurgeous/httpdisk"
	"golang.org/x/net/publicsuffix"
)

// read csv
func top1000() []string {
	f, _ := os.Open("tranco-top-1000.txt")
	defer f.Close()
	data, _ := ioutil.ReadAll(f)
	return strings.Fields(string(data))
}

// fetch top1000 and test caching
func main() {
	// args
	var noNet bool
	var one string
	flag.BoolVar(&noNet, "nonet", false, `If true, using the network will cause a panic`)
	flag.StringVar(&one, "one", "", `Just hit this one url instead of the top 1000`)
	flag.Parse()

	// setup the cache
	dir := filepath.Join(os.Getenv("HOME"), "top1000-httpdisk")
	hd := httpdisk.NewHTTPDisk(httpdisk.Options{Dir: dir})
	if noNet {
		hd.Transport = &panicTransport{}
	}

	// create http.Client
	client := http.Client{}
	client.Jar, _ = cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	client.Timeout = time.Second * 5
	client.Transport = &addHeadersTransport{Transport: hd}

	if one != "" {
		err := hit(&client, one)
		if err != nil {
			fmt.Printf("  FAIL %s type:%T v:%v\n", one, err, err)
			os.Exit(1)
		}
		fmt.Println("success!")
		os.Exit(0)
	}

	// make our requests
	for i, domain := range top1000() {
		url := fmt.Sprintf("http://%s", domain)
		fmt.Printf("%d/1000. %s\n", i+1, url)
		err := hit(&client, url)
		if err != nil {
			fmt.Printf("  FAIL %d %s type:%T v:%v\n", i+1, url, err, err)
		}
	}
}

func hit(client *http.Client, url string) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		panic(err)
	}

	_, err = client.Do(req)
	return err
}

//
// delegating Transport that adds default headers
//

type addHeadersTransport struct {
	Transport http.RoundTripper
}

func (t *addHeadersTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("Accept", "*/*")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/88.0.4324.182 Safari/537.36")
	return t.Transport.RoundTrip(req)
}

//
// Transport that panics, to ensure the network never gets hit
//

type panicTransport struct{}

func (t *panicTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	panic(fmt.Sprintf("panicTransport: uh oh, someone tried to use the network %s\n", req.URL))
}
