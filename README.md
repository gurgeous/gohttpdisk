![badge](https://github.com/gurgeous/gohttpdisk/workflows/Test/badge.svg)

### Overview

`gohttpdisk` will cache http responses on disk. Several of these already exist (see below) but this one is a bit different. The priority for `gohttpdisk` is to always cache on disk. It is not RFC compliant. It caches GET, POST and everything else. gohttpdisk is useful for crawling projects, to aggressively avoid extra http requests.

### Usage

Just plug gohttpdisk into an http.Client:

```go
hd := NewHTTPDisk(gohttpdisk.Options{})
client := http.Client{Transport: hd}
resp, err = client.Get("http://google.com")
...
```

Responses will be cached in `gohttpdisk`. The cache key is the md5 sum of the HTTP method, the normalized URL, and the request body. The path will be of the form `gohttpdisk/98/fa/1f08556382802ef7e26852c527c2`. Responses never expire and are never deleted by gohttpdisk. They will last forever and grow unbounded until manually deleted.

Note that HTTP headers are NOT used to calculate the cache key. This can be unintuitive for crawling projects that involve cookies or session state.

### Also See

Here are some other excellent caching libraries that you might want to check out. These generally act like traditional HTTP caches:

- [https://github.com/bxcodec/httpcache](https://github.com/bxcodec/httpcache)
- [https://github.com/gregjones/httpcache](https://github.com/gregjones/httpcache)
