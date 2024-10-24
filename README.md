# mouse-lib (an `automerge-go` sync library)

This library is a utility library for synchronising [automerge](https://automerge.org/) documents over HTTP using a `application/x-ndjson` protocol:

1. Both the request and response bodies contain newline-delimited json lines. The content-type is `application/x-ndjson; charset=utf-8`.
2. Each line looks like `{"event":"sync", "data":"<base64-encoded sync message>"}\n`
3. The server stays connected, continuously receiving messages and sending messages as they are ready on the document via either HTTP2 or well-behaved HTTP1.1 clients.
4. The client decides when to terminate the connection by observing the messages it receives, either:
    1. The response body is closed after the server detects that the request body is complete and no more messages are available.
    2. The client sees a sync message that meets its "termination check", which may indicate that the server matches the local state or that the local state contains all the remote head nodes. This can be used for local tools that need to perform a "one-shot" synchronisation on startup.
5. There's a broadcast capability that allows a server to serve changes from multiple clients on the same doc simultaneously or for a client to synchronise with multiple servers.

This library will be used to build a series of small peer-to-peer and distributed state utilities built on Automerge. The protocol above is easy to replicate in most languages, most importantly Go (in this repo) and Javascript.

## FAQ: Why not use the Automerge sync-server Websocket protocols?

The Automerge sync server <https://github.com/automerge/automerge-repo-sync-server> and related <https://github.com/automerge/automerge-connection> libraries are written in and generally use websockets to communicate. I wanted to try and utilise HTTP2 and concurrent HTTP1.1 to achieve a similar thing with Go.

This gives me a pure-go option with very few dependencies that I can trust to be stable and maintainable for a long time.

## Dependencies

This is purposefully built with only the Go standard library + `github.com/automerge/automerge-go`. This is to reduce maintenance burden for me.

## Testing

Unit tests, including Server and Client syncing.
