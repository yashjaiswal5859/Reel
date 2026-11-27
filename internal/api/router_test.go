package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"reel/internal/api"
	"reel/internal/logger"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHealthEndpoint(t *testing.T) {
	log := logger.New("error", "test")
	r := api.NewRouter(api.RouterDeps{Logger: log, Env: "test"})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var body map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "ok", body["status"])
	assert.Equal(t, "reel-api", body["service"])
}
