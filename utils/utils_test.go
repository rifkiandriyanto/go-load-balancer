package utils

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCountAttemptsDefault(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)

	assert.Equal(t, 1, CountAttempts(req))
}

func TestCountAttemptsWithValue(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	ctx := context.WithValue(req.Context(), Attempts, 3)
	req = req.WithContext(ctx)

	assert.Equal(t, 3, CountAttempts(req))
}

func TestCountRetriesDefault(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)

	assert.Equal(t, 0, CountRetries(req))
}

func TestCountRetriesWithValue(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	ctx := context.WithValue(req.Context(), Retries, 2)
	req = req.WithContext(ctx)

	assert.Equal(t, 2, CountRetries(req))
}
