package backend

import (
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"time"
)

type Backend struct {
	Addr           string
	URL            *url.URL
	alive          bool
	weight         int
	rProxy         *httputil.ReverseProxy
	mux            sync.RWMutex
	activeConn     int
	totalReq       int
	totalLatencyMs int
}

func New(addr string) *Backend {
	url, err := url.Parse(addr)
	if err != nil {
		panic(fmt.Sprintf("invalid backend address %q: %v", addr, err))
	}

	return &Backend{
		Addr:           addr,
		URL:            url,
		alive:          true,
		weight:         1,
		activeConn:     0,
		totalReq:       0,
		totalLatencyMs: 0,
	}
}

func (b *Backend) Address() string {
	return b.Addr
}

func (b *Backend) IsAlive() (alive bool) {
	b.mux.RLock()
	alive = b.alive
	b.mux.RUnlock()
	return
}

func (b *Backend) SetAlive(alive bool) {
	b.mux.Lock()
	b.alive = alive
	b.mux.Unlock()
}

func (b *Backend) Weight() int {
	b.mux.RLock()
	w := b.weight
	b.mux.RUnlock()
	return w
}

func (b *Backend) SetWeight(w int) {
	b.mux.Lock()
	b.weight = w
	b.mux.Unlock()
}

func (b *Backend) SetReverseProxy(proxy *httputil.ReverseProxy) {
	b.rProxy = proxy
}

func (b *Backend) ReverseProxy() *httputil.ReverseProxy {
	return b.rProxy
}

func (b *Backend) Serve(rw http.ResponseWriter, _ *http.Request) {
	ms := rand.Intn(250)
	time.Sleep(time.Duration(ms) * time.Millisecond)

	b.mux.Lock()
	b.activeConn += 1
	b.totalReq += 1
	b.totalLatencyMs += ms
	b.mux.Unlock()

	defer func() {
		b.mux.Lock()
		b.activeConn -= 1
		b.mux.Unlock()
	}()

	fmt.Fprintf(rw, "(%s) Returned response in %d(ms)\n", b.Address(), ms)
}

func (b *Backend) ActiveConnections() int {
	b.mux.RLock()
	n := b.activeConn
	b.mux.RUnlock()
	return n
}

func (b *Backend) TotalRequests() int {
	b.mux.RLock()
	n := b.totalReq
	b.mux.RUnlock()
	return n
}

func (b *Backend) AverageLatency() int {
	b.mux.RLock()
	defer b.mux.RUnlock()

	if b.totalReq == 0 {
		return 75
	}
	return b.totalLatencyMs / b.totalReq
}

func (b *Backend) RequestHandler() func(rw http.ResponseWriter, req *http.Request) {
	return func(rw http.ResponseWriter, req *http.Request) {
		b.Serve(rw, req)
	}
}
