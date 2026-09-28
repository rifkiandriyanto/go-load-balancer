# Balancer
A simple load balancer written in Go.

> **Origin:** This repository is a fork/clone of [`github.com/wfgilman/balancer`](https://github.com/wfgilman/balancer),
> kept up-to-date and extended with the improvements listed below.

### Features
* [x] Testing (`go test ./...`)
* [x] Round Robin Method
* [x] Health Checks
* [x] Fewest Active Connections Method
* [x] Least Latency Method
* [x] Weighted Round Robin

### Improvements over the original
* **Weighted Round Robin** — new smooth (nginx-style) algorithm and `--weights` flag so
  backends receive traffic proportional to their weight.
* **Fully working test suite** — the original tests referenced types that no longer existed
  (`backend.WebServer`, `LoadBalancer`); rewritten and expanded to cover every algorithm,
  unhealthy-backend skipping, proxy forwarding, and health checks.
* **Thread safety** — backend metrics (`activeConn`, `totalReq`, `totalLatencyMs`) are now
  guarded by a mutex; fixed the round-robin cursor and Weighted Round Robin state to be
  safe under concurrency (`go test -race ./...` passes).
* **Correct retry state** — `proxy.ErrorHandler` now takes `*pool.Pool`; previously it
  received a copy, so the pool algorithm reset its cursor/weights on every retry.
* **Modernized dependencies** — Go `1.18 → 1.22`, `testify` `v1.8 → v1.12`, removed unused
  `golang.org/x/sync`, dropped deprecated `rand.Seed`.
* **Style & error handling** — applied the single-handling-rule (log *or* return), cleaner
  early-return control flow, and health checks that skip dead backends.
* **`AGENTS.md`** — guidance + installed skills for AI coding agents working on this repo.

### How it works
![Schematic](/assets/lb.png)

On start-up, the application creates `n` backend web servers in the `5000` block on
the `localhost` port from `5000` to `500N`. Each backend conforms to the `Server` interface.
The backend superficially responds to `GET` requests with its address and the response time.
Response times are randomly chosen between 0 and 250ms. Each backend also carries a
`weight` (default `1`) used by the Weighted Round Robin algorithm.

Each backend is assigned a single host reverse proxy. The purpose of the proxy in this
application is to handle backend failure gracefully within the server pool. In Go, the
reverse proxy has an `ErrorHandler` method which can be customized to retry requests or
propagate issues with its assigned backend to the server pool.

Each backend is assigned to the server pool of the load balancer. The pool is responsible
for implementing the server rotation algorithms. An HTTP server is created and implements
a response handler for the pool that decides which backend to send the request to.

The `main` function runs a health check on an interval which makes a TCP connection to each backend
server to verify it is alive, then sets the status of each backend through the `Server` interface
accordingly.

### Usage
Clone the repository and navigate to the root of the project and run `go run .`
The application takes four flags:
```
--n         The number of backend servers to start. Defaults to 5.
--port      The port number on which to start the load balancer. This will
            be the port you'd call with cURL
--algo      The balancing algorithm to use. Options are:
            "alwaysfirst"        Takes the first server in the slice
            "roundrobin"         Takes the next healthy server sequentially
            "leastlatency"       Takes the server with the lowest average response time
            "fewestconn"         Takes the server with the least active connections
            "weightedroundrobin" Smooth weighted round robin, honors each backend weight
--weights   Comma-separated weights applied to the backends (used by
            "weightedroundrobin"). Cycles if fewer weights than backends are
            given. Defaults to a repeating 1,2,3 pattern. Example: --weights 5,1,1
```
In another window, run the following cURL command:
```
$ curl http://localhost:8000
(localhost:5000) Returned response in 46(ms)
(localhost:5001) Returned response in 44(ms)
(localhost:5002) Returned response in 175(ms)
(localhost:5003) Returned response in 197(ms)
(localhost:5004) Returned response in 121(ms)
(localhost:5000) Returned response in 103(ms)
```

Weighted Round Robin demo with `3:1` weights across two backends:
```
$ go run . --algo weightedroundrobin --weights 3,1
$ curl http://localhost:8000   # repeat several times
```

### Round Robin Algorithm
![Schematic](/assets/go-balancer.gif)

### Credits
The design of this application is inspired and informed by the content and code
in the online resources below.

* Load Balancer: https://kasvith.me/posts/lets-create-a-simple-lb-go/
* Load Balancer: https://betterprogramming.pub/building-a-load-balancer-in-go-3da3c7c46f30
* Graceful Server Shutdown: https://www.rudderstack.com/blog/implementing-graceful-shutdown-in-go
* Reverse Proxy: https://blog.joshsoftware.com/2021/05/25/simple-and-powerful-reverseproxy-in-go/
* Enums: https://www.sohamkamani.com/golang/enums/
