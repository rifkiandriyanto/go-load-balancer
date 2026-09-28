package pool

import (
	"errors"
	"log"
	"math"
	"net"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wfgilman/balancer/server"
)

const (
	AlwaysFirst        string = "alwaysfirst"
	RoundRobin                = "roundrobin"
	LeastLatency              = "leastlatency"
	FewestConn                = "fewestconn"
	WeightedRoundRobin        = "weightedroundrobin"
)

type Pool struct {
	Current        uint64
	Servers        []server.Server
	Algorithm      string
	mu             sync.Mutex
	currentWeights []int
}

func New(algorithm string) *Pool {
	return &Pool{
		Servers:        []server.Server{},
		Algorithm:      algorithm,
		currentWeights: []int{},
	}
}

func (p *Pool) AddServer(server server.Server) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.Servers = append(p.Servers, server)
	p.currentWeights = append(p.currentWeights, 0)
}

func (p *Pool) RequestHandler() func(rw http.ResponseWriter, req *http.Request) {
	return func(rw http.ResponseWriter, req *http.Request) {
		p.Serve(rw, req)
	}
}

func (p *Pool) Serve(rw http.ResponseWriter, req *http.Request) {
	server := p.GetNextServer()
	proxy := server.ReverseProxy()
	proxy.ServeHTTP(rw, req)
}

func (p *Pool) GetNextServer() server.Server {
	switch p.Algorithm {
	case AlwaysFirst:
		return p.Servers[0]
	case RoundRobin:
		return p.roundRobin()
	case LeastLatency:
		return p.leastLatency()
	case FewestConn:
		return p.fewestConn()
	case WeightedRoundRobin:
		return p.weightedRoundRobin()
	}
	panic("No backends exist")
}

func (p *Pool) roundRobin() server.Server {
	nextIndex := int(atomic.AddUint64(&p.Current, uint64(1)) % uint64(len(p.Servers)))
	length := len(p.Servers) + nextIndex
	for i := nextIndex; i < length; i++ {
		index := i % len(p.Servers)
		if p.Servers[index].IsAlive() {
			if i != nextIndex {
				atomic.StoreUint64(&p.Current, uint64(index))
			}
			return p.Servers[index]
		}
	}
	panic("No healthy backends exist")
}

func (p *Pool) leastLatency() server.Server {
	var selected server.Server
	min := math.MaxInt

	for _, s := range p.Servers {
		latency := s.AverageLatency()
		if s.IsAlive() && latency < min {
			min = latency
			selected = s
		}
	}
	return selected
}

func (p *Pool) fewestConn() server.Server {
	var selected server.Server
	min := math.MaxInt

	for _, s := range p.Servers {
		activeConn := s.ActiveConnections()
		if s.IsAlive() && activeConn < min {
			min = activeConn
			selected = s
		}
	}
	return selected
}

// weightedRoundRobin implements smooth weighted round robin (nginx-style):
// each request raises a server's current weight by its static weight, then the
// server with the highest current weight is chosen and reduced by the total.
func (p *Pool) weightedRoundRobin() server.Server {
	p.mu.Lock()
	defer p.mu.Unlock()

	var selected server.Server
	selectedIndex := -1
	total := 0

	for i, s := range p.Servers {
		if !s.IsAlive() {
			continue
		}

		weight := s.Weight()
		p.currentWeights[i] += weight
		total += weight

		if selectedIndex == -1 || p.currentWeights[i] > p.currentWeights[selectedIndex] {
			selectedIndex = i
			selected = s
		}
	}

	if selectedIndex == -1 {
		panic("No healthy backends exist")
	}

	p.currentWeights[selectedIndex] -= total
	return selected
}

func (p *Pool) GetServer(targetAddr string) (server.Server, error) {
	for _, s := range p.Servers {
		if s.Address() == targetAddr {
			return s, nil
		}
	}
	return nil, errors.New("Server not found")
}

func (p *Pool) HealthCheck() {
	for _, s := range p.Servers {
		if !s.IsAlive() {
			continue
		}

		alive := isServerAlive(s)
		s.SetAlive(alive)
		if !alive {
			log.Printf("(%s) is down\n", s.Address())
		}
	}
}

func isServerAlive(s server.Server) bool {
	targetURL := &url.URL{
		Scheme: "http",
		Host:   s.Address(),
	}

	conn, err := net.DialTimeout("tcp", targetURL.Host, 2*time.Second)
	if err != nil {
		return false
	}
	defer conn.Close()

	return true
}

func (p *Pool) ServerStats() {
	for _, s := range p.Servers {
		log.Printf(
			"(%s) Alive %v, Active %d, Total %d, Latency %d(ms), Weight %d\n",
			s.Address(),
			s.IsAlive(),
			s.ActiveConnections(),
			s.TotalRequests(),
			s.AverageLatency(),
			s.Weight(),
		)
	}
}
