// Package embed provides optional semantic search via pgvector + an external
// OpenAI-compatible embeddings endpoint (text-embeddings-inference, vLLM,
// OpenAI, LocalAI, Ollama, etc).
//
// The feature is entirely opt-in: by default the search handler uses the
// existing LIKE-based engine. When the workspace setting search_mode is set
// to "semantic" AND pgvector is available AND EMBEDDING_API_URL is configured,
// the search handler dispatches to the semantic engine instead.
package embed

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client talks to an OpenAI-compatible /v1/embeddings endpoint.
type Client struct {
	baseURL string
	apiKey  string
	model   string
	http    *http.Client
}

// NewClient returns nil if baseURL is empty (semantic search disabled).
func NewClient(baseURL, apiKey, model string) *Client {
	if baseURL == "" {
		return nil
	}
	if model == "" {
		model = "BAAI/bge-small-en-v1.5"
	}
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		model:   model,
		http:    &http.Client{Timeout: 30 * time.Second},
	}
}

// Model returns the configured model identifier.
func (c *Client) Model() string {
	if c == nil {
		return ""
	}
	return c.model
}

type embedReq struct {
	Input []string `json:"input"`
	Model string   `json:"model"`
}

type embedResp struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// Embed returns one vector per input string. Inputs are batched in a single request.
func (c *Client) Embed(ctx context.Context, inputs []string) ([][]float32, error) {
	if c == nil {
		return nil, fmt.Errorf("embed: client not configured")
	}
	if len(inputs) == 0 {
		return nil, nil
	}

	body, err := json.Marshal(embedReq{Input: inputs, Model: c.model})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/v1/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("embed request: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("embed http %d: %s", resp.StatusCode, truncate(string(respBody), 200))
	}

	var out embedResp
	if err := json.Unmarshal(respBody, &out); err != nil {
		return nil, fmt.Errorf("embed decode: %w (body: %s)", err, truncate(string(respBody), 200))
	}
	if out.Error != nil {
		return nil, fmt.Errorf("embed api: %s", out.Error.Message)
	}
	if len(out.Data) != len(inputs) {
		return nil, fmt.Errorf("embed: expected %d vectors, got %d", len(inputs), len(out.Data))
	}

	vecs := make([][]float32, len(out.Data))
	for i, d := range out.Data {
		vecs[i] = d.Embedding
	}
	return vecs, nil
}

// Health pings the endpoint with a tiny request to verify it works.
// Returns the embedding dimension on success.
func (c *Client) Health(ctx context.Context) (int, error) {
	if c == nil {
		return 0, fmt.Errorf("embed: client not configured")
	}
	vecs, err := c.Embed(ctx, []string{"ping"})
	if err != nil {
		return 0, err
	}
	if len(vecs) == 0 || len(vecs[0]) == 0 {
		return 0, fmt.Errorf("embed: empty vector returned")
	}
	return len(vecs[0]), nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
