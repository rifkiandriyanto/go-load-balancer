package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"load-balancer/backend"
	"load-balancer/pool"
	"load-balancer/proxy"
)

var serverPool *pool.Pool

func healthCheck() {
	t := time.NewTicker(15 * time.Second)
	for {
		select {
		case <-t.C:
			log.Println("Health check started")
			serverPool.HealthCheck()
			log.Println("Health check complete")
		}
	}
}

func serverStats() {
	t := time.NewTicker(20 * time.Minute)
	for {
		select {
		case <-t.C:
			log.Println("Server Report...")
			serverPool.ServerStats()
		}
	}
}

func parseWeights(raw string, numBackends int) ([]int, error) {
	weights := make([]int, 0, numBackends)
	if raw == "" {
		for i := 0; i < numBackends; i++ {
			weights = append(weights, i%3+1)
		}
		return weights, nil
	}

	parts := strings.Split(raw, ",")
	for i := 0; i < numBackends; i++ {
		w, err := strconv.Atoi(parts[i%len(parts)])
		if err != nil {
			return nil, fmt.Errorf("invalid weight %q: %w", parts[i%len(parts)], err)
		}
		weights = append(weights, w)
	}
	return weights, nil
}

func main() {
	// Parse startup parameters from command line.
	numBackends := flag.Int("n", 5, "Enter number of backend servers")
	port := flag.Int("port", 8000, "Enter port number for load balancer")
	algo := flag.String("algo", "alwaysfirst", "Balancing algorithm")
	weightsRaw := flag.String("weights", "", "Comma-separated weights for weighted round robin (cycles if fewer than backends)")
	flag.Parse()

	weights, err := parseWeights(*weightsRaw, *numBackends)
	if err != nil {
		log.Fatal(err)
	}

	serverPool = pool.New(*algo)

	for i := 0; i < *numBackends; i++ {
		// Create backend server.
		addr := fmt.Sprintf("localhost:500%d", i)
		be := backend.New(addr)
		be.SetWeight(weights[i])
		beServer := http.Server{
			Addr:    addr,
			Handler: http.HandlerFunc(be.RequestHandler()),
		}

		// Start backend server.
		log.Printf("(%s) Backend Server started\n", be.Address())
		go beServer.ListenAndServe()

		// Randomly crash a server
		if i%*numBackends == 1 {
			time.AfterFunc(25*time.Second, func() {
				log.Printf("(%s) Backend Server crashed\n", be.Address())
				beServer.Shutdown(context.Background())
			})
		}

		// Create a ReverseProxy pointing to backend server.
		p := proxy.New(be.Address())
		p.ErrorHandler = proxy.ErrorHandler(p, serverPool, be)
		be.SetReverseProxy(p)

		// Add the backend to the server pool.
		serverPool.AddServer(be)
	}

	// Create the load balancer server.
	server := http.Server{
		Addr:    fmt.Sprintf("localhost:%d", *port),
		Handler: http.HandlerFunc(serverPool.RequestHandler()),
	}

	go healthCheck()
	go serverStats()

	// Start the load balancer.
	log.Printf("(%s) Load Balancer started\n", server.Addr)
	server.ListenAndServe()
}
