package main

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gurgeous/gohttpdisk"
	"github.com/spf13/pflag"
)

//
// Print status for a gohttpdisk request
//

type Args struct {
	dir     string
	nohosts bool
	status  bool
	u       *url.URL
}

func main() {
	// cli
	args, err := cli()
	if err != nil {
		msg := err.Error()
		if msg != "" {
			fmt.Fprintf(os.Stderr, "gohttpdisk: %s\n", msg)
		}
		fmt.Fprintln(os.Stderr, "gohttpdisk: try 'gohttpdisk --help' for more information")
		os.Exit(1)
	}
	if !args.status {
		fmt.Println("sorry, the only thing we support is --status")
		os.Exit(1)
	}

	// go!
	hd := gohttpdisk.NewHTTPDisk(gohttpdisk.Options{Dir: args.dir, NoHosts: args.nohosts})

	// status
	if args.status {
		status, err := status(hd, args.u)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error %s\n", err.Error())
			os.Exit(1)
		}

		fmt.Printf("url: %q\n", status.URL)
		fmt.Printf("status: %q\n", status.Status)
		fmt.Printf("key: %q\n", status.Key)
		fmt.Printf("digest: %q\n", status.Digest)
		fmt.Printf("path: %q\n", status.Path)
		if status.Age > 0 {
			fmt.Printf("age: %q\n", status.Age.Truncate(time.Second))
		}

	}
}

func status(hd *gohttpdisk.HTTPDisk, u *url.URL) (*gohttpdisk.Status, error) {
	request, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return nil, err
	}
	status, err := hd.Status(request)
	if err != nil {
		return nil, err
	}
	return status, nil
}

//
// cli
//

func cli() (*Args, error) {
	// no arguments, give a hint
	if len(os.Args) == 1 {
		return nil, errors.New("")
	}

	// get ready
	cli := pflag.NewFlagSet("gohttpdisk", pflag.ContinueOnError)
	defaultDir := filepath.Join(os.Getenv("HOME"), "gohttpdisk")
	dir := cli.String("dir", defaultDir, "cache directory")
	nohosts := cli.Bool("nohosts", false, "don't include hostname in cache path")
	status := cli.Bool("status", false, "show status for a url in the cache")
	help := cli.BoolP("help", "h", false, "show this help")

	// parse, handle --help
	if err := cli.Parse(os.Args[1:]); err != nil {
		return nil, err
	}
	if *help {
		fmt.Println("gohttpdisk [options] [url]")
		cli.PrintDefaults()
		os.Exit(0)
	}

	// url arg
	if cli.NArg() == 0 {
		return nil, errors.New("no URL specified")
	}
	if cli.NArg() > 1 {
		return nil, errors.New("more than one URL specified")
	}
	urlString := cli.Arg(0)
	if !(strings.HasPrefix(urlString, "http:") || strings.HasPrefix(urlString, "https:")) {
		if strings.Contains(urlString, "://") {
			return nil, errors.New("only http/https supported")
		}
		urlString = "https://" + urlString
	}
	u, err := url.Parse(urlString)
	if err != nil {
		return nil, err
	}

	return &Args{
		dir:     *dir,
		nohosts: *nohosts,
		status:  *status,
		u:       u,
	}, nil
}
