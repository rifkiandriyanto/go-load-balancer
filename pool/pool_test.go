package pool

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/wfgilman/balancer/server"
)

type stubServer struct {
	addr       string
	alive      bool
	weight     int
	activeConn int
	totalReq   int
	avgLatency int
	rProxy     *httputil.ReverseProxy
}

func newStubServer(addr string, mutate func(*stubServer)) *stubServer {
	s := &stubServer{
		addr:   addr,
		alive:  true,
		weight: 1,
	}
	if mutate != nil {
		mutate(s)
	}
	return s
}

func (s *stubServer) Address() string                          { return s.addr }
func (s *stubServer) IsAlive() bool                            { return s.alive }
func (s *stubServer) SetAlive(alive bool)                      { s.alive = alive }
func (s *stubServer) Weight() int                              { return s.weight }
func (s *stubServer) SetWeight(w int)                          { s.weight = w }
func (s *stubServer) ActiveConnections() int                   { return s.activeConn }
func (s *stubServer) TotalRequests() int                       { return s.totalReq }
func (s *stubServer) AverageLatency() int                      { return s.avgLatency }
func (s *stubServer) SetReverseProxy(p *httputil.ReverseProxy) { s.rProxy = p }
func (s *stubServer) ReverseProxy() *httputil.ReverseProxy     { return s.rProxy }
func (s *stubServer) Serve(rw http.ResponseWriter, _ *http.Request) {
	fmt.Fprintf(rw, "hello from %s", s.addr)
}

var _ server.Server = (*stubServer)(nil)

func newPool(algo string) *Pool {
	return New(algo)
}

func TestAddServer(t *testing.T) {
	p := newPool(AlwaysFirst)
	srv := newStubServer("localhost:5000", nil)

	p.AddServer(srv)

	assert.Len(t, p.Servers, 1)
	assert.Same(t, srv, p.Servers[0])
}

func TestAlwaysFirst(t *testing.T) {
	p := newPool(AlwaysFirst)
	for i := 0; i < 3; i++ {
		p.AddServer(newStubServer(fmt.Sprintf("localhost:500%d", i), nil))
	}

	for i := 0; i < 3; i++ {
		assert.Equal(t, "localhost:5000", p.GetNextServer().Address())
	}
}

func TestRoundRobin(t *testing.T) {
	p := newPool(RoundRobin)
	for i := 0; i < 4; i++ {
		p.AddServer(newStubServer(fmt.Sprintf("localhost:500%d", i), nil))
	}
	p.Current = uint64(len(p.Servers) - 1)

	want := []string{"localhost:5000", "localhost:5001", "localhost:5002", "localhost:5003"}
	for i := 0; i < 2; i++ {
		for _, addr := range want {
			assert.Equal(t, addr, p.GetNextServer().Address())
		}
	}
}

func TestRoundRobinSkipsUnhealthy(t *testing.T) {
	p := newPool(RoundRobin)
	for i := 0; i < 4; i++ {
		p.AddServer(newStubServer(fmt.Sprintf("localhost:500%d", i), nil))
	}
	p.Servers[1].SetAlive(false)
	p.Servers[3].SetAlive(false)
	p.Current = uint64(len(p.Servers) - 1)

	for i := 0; i < 4; i++ {
		got := p.GetNextServer().Address()
		assert.NotEqual(t, "localhost:5001", got)
		assert.NotEqual(t, "localhost:5003", got)
	}
}

func TestFewestConn(t *testing.T) {
	p := newPool(FewestConn)
	p.AddServer(newStubServer("busy", func(s *stubServer) { s.activeConn = 9 }))
	p.AddServer(newStubServer("free", func(s *stubServer) { s.activeConn = 1 }))
	p.AddServer(newStubServer("busier", func(s *stubServer) { s.activeConn = 4 }))

	assert.Equal(t, "free", p.GetNextServer().Address())
}

func TestFewestConnSkipsUnhealthy(t *testing.T) {
	p := newPool(FewestConn)
	p.AddServer(newStubServer("free", func(s *stubServer) { s.activeConn = 1 }))
	p.AddServer(newStubServer("down", func(s *stubServer) { s.alive = false; s.activeConn = 0 }))

	assert.Equal(t, "free", p.GetNextServer().Address())
}

func TestLeastLatency(t *testing.T) {
	p := newPool(LeastLatency)
	p.AddServer(newStubServer("slow", func(s *stubServer) { s.avgLatency = 250 }))
	p.AddServer(newStubServer("fast", func(s *stubServer) { s.avgLatency = 5 }))

	assert.Equal(t, "fast", p.GetNextServer().Address())
}

func TestLeastLatencySkipsUnhealthy(t *testing.T) {
	p := newPool(LeastLatency)
	p.AddServer(newStubServer("down", func(s *stubServer) { s.alive = false; s.avgLatency = 1 }))
	p.AddServer(newStubServer("slow", func(s *stubServer) { s.avgLatency = 250 }))

	assert.Equal(t, "slow", p.GetNextServer().Address())
}

func TestWeightedRoundRobin(t *testing.T) {
	p := newPool(WeightedRoundRobin)
	p.AddServer(newStubServer("a", func(s *stubServer) { s.weight = 3 }))
	p.AddServer(newStubServer("b", func(s *stubServer) { s.weight = 2 }))
	p.AddServer(newStubServer("c", func(s *stubServer) { s.weight = 1 }))

	counts := map[string]int{}
	for i := 0; i < 60; i++ {
		counts[p.GetNextServer().Address()]++
	}

	// Smooth WRR over weights 3:2:1 yields an exact 30/20/10 split per 60 picks.
	assert.Equal(t, 30, counts["a"])
	assert.Equal(t, 20, counts["b"])
	assert.Equal(t, 10, counts["c"])
}

func TestWeightedRoundRobinSkipsUnhealthy(t *testing.T) {
	p := newPool(WeightedRoundRobin)
	p.AddServer(newStubServer("down", func(s *stubServer) { s.alive = false; s.weight = 100 }))
	p.AddServer(newStubServer("b", func(s *stubServer) { s.weight = 1 }))
	p.AddServer(newStubServer("c", func(s *stubServer) { s.weight = 1 }))

	for i := 0; i < 10; i++ {
		assert.NotEqual(t, "down", p.GetNextServer().Address())
	}
}

func TestGetServer(t *testing.T) {
	p := newPool(AlwaysFirst)
	p.AddServer(newStubServer("localhost:5000", nil))

	srv, err := p.GetServer("localhost:5000")
	assert.NoError(t, err)
	assert.Equal(t, "localhost:5000", srv.Address())

	_, err = p.GetServer("localhost:9999")
	assert.Error(t, err)
}

func TestServe(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, "hello from upstream")
	}))
	defer upstream.Close()

	targetURL, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatal(err)
	}
	addr := targetURL.Host

	p := newPool(AlwaysFirst)
	p.AddServer(newStubServer(addr, func(s *stubServer) {
		s.rProxy = httputil.NewSingleHostReverseProxy(targetURL)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	p.Serve(rr, req)

	body, err := io.ReadAll(rr.Result().Body)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "hello from upstream", string(body))
}

func TestIsServerAliveUp(t *testing.T) {
	ln, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	assert.True(t, isServerAlive(newStubServer(ln.Addr().String(), nil)))
}

func TestIsServerAliveDown(t *testing.T) {
	ln, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close()

	assert.False(t, isServerAlive(newStubServer(addr, nil)))
}
