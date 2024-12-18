# automerge-ndjson-sync (an `automerge-go` sync library)

![GitHub License](https://img.shields.io/github/license/astromechza/automerge-ndjson-sync)
![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/astromechza/automerge-ndjson-sync)
![GitHub Actions Workflow Status](https://img.shields.io/github/actions/workflow/status/astromechza/automerge-ndjson-sync/ci.yaml)

This library is a utility library for synchronising [automerge](https://automerge.org/) documents over HTTP using a `application/x-ndjson` protocol:

1. Both the request and response bodies contain newline-delimited json lines. The content-type is `application/x-ndjson; charset=utf-8`.
2. Each line looks like `{"event":"sync", "data":"<base64-encoded sync message>"}\n`
3. The server stays connected, continuously receiving messages and sending messages as they are ready on the document via either HTTP2 or well-behaved HTTP1.1 clients.
4. The client decides when to terminate the connection by observing the messages it receives, either:
    1. The response body is closed after the server detects that the request body is complete and no more messages are available.
    2. The client sees a sync message that meets its "termination check", which may indicate that the server matches the local state or that the local state contains all the remote head nodes. This can be used for local tools that need to perform a "one-shot" synchronisation on startup.
5. There's a broadcast capability that allows a server to serve changes from multiple clients on the same doc simultaneously or for a client to synchronise with multiple servers.
6. The client supports HTTP redirect behavior so that servers can implement rudimentary partitioning and balancing of requests.

This library will be used to build a series of small peer-to-peer and distributed state utilities built on Automerge. The protocol above is easy to replicate in most languages, most importantly Go (in this repo) and Javascript.

## FAQ: Why not use the Automerge sync-server Websocket protocols?

The Automerge sync server <https://github.com/automerge/automerge-repo-sync-server> and related <https://github.com/automerge/automerge-connection> libraries are written in and generally use websockets to communicate. I wanted to try and utilise HTTP2 and concurrent HTTP1.1 to achieve a similar thing with Go.

This gives me a pure-go option with very few dependencies that I can trust to be stable and maintainable for a long time.

## FAQ: Can you give me an example over the wire?

By executing `go run ./examples/server/` in one terminal, and `DOC_ID=example go run ./examples/http2writer` in another terminal, I can then execute a raw `curl` request in a 3rd terminal to follow the stream over HTTPS. I send an empty sync message to start and observe the following:

```
$ curl -k -v -X PUT https://localhost:8080/example -d '{"event":"sync","data":"QgAAAQAAAA=="}' -H 'Content-Type: application/x-ndjson'
...
> PUT /example HTTP/2
> Host: localhost:8080
> User-Agent: curl/8.5.0
> Accept: */*
> Content-Type: application/x-ndjson
> Content-Length: 38

< HTTP/2 200 
< content-type: application/x-ndjson; charset=utf-8
< date: Sat, 26 Oct 2024 09:56:23 GMT
< 
{"event":"sync","data":"QgFbkqa2LT<snip>9CqZQD6wA="}
{"event":"sync","data":"QgEVJTDZHR<snip>F/AH+BAQ=="}
{"event":"sync","data":"QgG3qWH1RH<snip>8BfwB/ggE="}
...
```

Until I decide to hang up the connection with Ctrl-C. And this works perfectly fine with HTTP1.1 too:

```
$ curl -k -i -X PUT https://localhost:8080/example -d '{"event":"sync","data":"QgAAAQAAAA=="}' -H 'Content-Type: application/x-ndjson' --http1.1
< HTTP/1.1 200 OK
< Content-Type: application/x-ndjson; charset=utf-8
< Date: Sat, 26 Oct 2024 11:32:52 GMT
< Transfer-Encoding: chunked

{"event":"sync","data":"QgFbkqa2LT<snip>9CqZQD6wA="}
{"event":"sync","data":"QgEVJTDZHR<snip>F/AH+BAQ=="}
{"event":"sync","data":"QgG3qWH1RH<snip>8BfwB/ggE="}
...
```

## FAQ: Why do the examples use HTTPS, do I need to use HTTPS?

The examples include an HTTP2 client, so the server has a self-signed certificate so that it can present HTTP2 over HTTPS.
The client can still use HTTP1.1 as seen in the curl example above or the http1follower example.

## Dependencies

This is purposefully built with only the Go standard library + `github.com/automerge/automerge-go`. This is to reduce maintenance burden for me.

## Testing

Unit tests, including Server and Client syncing, are executed through either `make test` or the Github Actions CI.
