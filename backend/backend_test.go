package backend

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	addr := "http://localhost:4000"

	expectedURL, _ := url.Parse(addr)
	backend := New(addr)

	assert.Equal(t, expectedURL, backend.URL)
	assert.Equal(t, addr, backend.Addr)
	assert.True(t, backend.IsAlive())
	assert.Equal(t, 1, backend.Weight())
}

func TestAddress(t *testing.T) {
	addr := "https://www.google.com"

	assert.Equal(t, addr, New(addr).Address())
}

func TestIsAlive(t *testing.T) {
	backend := New("http://localhost:4000")

	assert.True(t, backend.IsAlive())
}

func TestSetAlive(t *testing.T) {
	backend := New("http://localhost:4000")
	backend.SetAlive(false)

	assert.False(t, backend.IsAlive())
}

func TestWeight(t *testing.T) {
	backend := New("http://localhost:4000")
	backend.SetWeight(4)

	assert.Equal(t, 4, backend.Weight())
}

func TestServe(t *testing.T) {
	backend := New("http://localhost:4000")

	req, err := http.NewRequest(http.MethodGet, "/", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(backend.Serve)
	handler.ServeHTTP(rr, req)

	assert.NotEmpty(t, rr.Body.String())
	assert.Equal(t, 0, backend.ActiveConnections())
	assert.Equal(t, 1, backend.TotalRequests())
	assert.Greater(t, backend.AverageLatency(), 0)
}
