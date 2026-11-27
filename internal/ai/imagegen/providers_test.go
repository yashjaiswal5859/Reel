package imagegen_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"reel/internal/ai/imagegen"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReplicate_Generate(t *testing.T) {
	var srvURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/img.png" {
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write([]byte("PNG-BYTES"))
			return
		}
		assert.Equal(t, "Bearer rep-key", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": "succeeded",
			"output": []string{srvURL + "/img.png"},
		})
	}))
	defer srv.Close()
	srvURL = srv.URL

	client := imagegen.NewReplicateWithBaseURL(srv.URL, "rep-key", "black-forest-labs/flux-1.1-pro", srv.Client())
	resp, err := client.Generate(context.Background(), imagegen.GenerateRequest{Prompt: "A brave boy"})
	require.NoError(t, err)
	assert.Equal(t, "replicate", resp.Provider)
	assert.Equal(t, []byte("PNG-BYTES"), resp.ImageBytes)
}

func TestFal_Generate(t *testing.T) {
	var srvURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/out.png" {
			_, _ = w.Write([]byte("FAL-PNG"))
			return
		}
		assert.Equal(t, "Key fal-key", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"images": []map[string]any{{"url": srvURL + "/out.png", "content_type": "image/png"}},
		})
	}))
	defer srv.Close()
	srvURL = srv.URL

	client := imagegen.NewFalWithBaseURL(srv.URL, "fal-key", "fal-ai/flux-pro/v1.1", srv.Client())
	resp, err := client.Generate(context.Background(), imagegen.GenerateRequest{Prompt: "A forest"})
	require.NoError(t, err)
	assert.Equal(t, "fal", resp.Provider)
	assert.Equal(t, []byte("FAL-PNG"), resp.ImageBytes)
}

func TestOpenAI_Generate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer oai-key", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{{"b64_json": base64.StdEncoding.EncodeToString([]byte("DALLE-PNG"))}},
		})
	}))
	defer srv.Close()

	client := imagegen.NewOpenAIWithEndpoint(srv.URL, "oai-key", "dall-e-3", srv.Client())
	resp, err := client.Generate(context.Background(), imagegen.GenerateRequest{Prompt: "A castle", Width: 1024, Height: 1792})
	require.NoError(t, err)
	assert.Equal(t, "openai", resp.Provider)
	assert.Equal(t, []byte("DALLE-PNG"), resp.ImageBytes)
}
