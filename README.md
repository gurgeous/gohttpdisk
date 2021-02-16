### Overview

![badge](https://github.com/gurgeous/httpdisk/workflows/Test/badge.svg)

`httpdisk` will cache http responses on disk. Several of these already exist (see below) but this one is a bit different. The priority for `httpdisk` is to always cache on disk. It is not RFC compliant. It caches GET, POST and everything else. httpdisk is useful for crawling projects, to aggressively avoid extra http requests.

### Getting Started

Just plug httpdisk into an http.Client:

```go
hd := NewHTTPDisk("/tmp/httpdisk")
client := http.Client{Transport: hd}
resp, err = client.Get("http://google.com")
...
```

Responses will be cached in `/tmp/httpdisk`. The cache key is the md5 sum of the HTTP method, the normalized URL, and the request body. The path will be of the form `/tmp/httpdisk/98/fa/1f08556382802ef7e26852c527c2`. Responses never expire and are never deleted by httpdisk. They will last forever and grow unbounded until manually deleted.

Note that HTTP headers are NOT used to calculate the cache key. This can be unintuitive for crawling projects that involve cookies or session state.

### Also See

Here are other caching libraries that may meet your needs. These generally act more like normal HTTP caches:

- [https://github.com/bxcodec/httpcache](https://github.com/bxcodec/httpcache)
- [https://github.com/gregjones/httpcache](https://github.com/gregjones/httpcache)
