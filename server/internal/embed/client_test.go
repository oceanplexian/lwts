package embed

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewClient_NilOnEmptyURL(t *testing.T) {
	if NewClient("", "", "") != nil {
		t.Fatal("expected nil client for empty URL")
	}
}

func TestNewClient_DefaultModel(t *testing.T) {
	c := NewClient("http://x", "", "")
	if c.Model() != "BAAI/bge-small-en-v1.5" {
		t.Fatalf("default model: %s", c.Model())
	}
}

func TestEmbed_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req embedReq
		_ = json.NewDecoder(r.Body).Decode(&req)
		resp := embedResp{}
		for range req.Input {
			resp.Data = append(resp.Data, struct {
				Embedding []float32 `json:"embedding"`
			}{Embedding: []float32{0.1, 0.2, 0.3}})
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "", "")
	vecs, err := c.Embed(context.Background(), []string{"a", "b"})
	if err != nil {
		t.Fatal(err)
	}
	if len(vecs) != 2 || len(vecs[0]) != 3 {
		t.Fatalf("shape: %v", vecs)
	}
}

func TestEmbed_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "", "")
	if _, err := c.Embed(context.Background(), []string{"x"}); err == nil {
		t.Fatal("expected error")
	}
}

func TestEmbed_AuthHeader(t *testing.T) {
	gotAuth := ""
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_ = json.NewEncoder(w).Encode(embedResp{Data: []struct {
			Embedding []float32 `json:"embedding"`
		}{{Embedding: []float32{0.1}}}})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "secret", "")
	if _, err := c.Embed(context.Background(), []string{"x"}); err != nil {
		t.Fatal(err)
	}
	if gotAuth != "Bearer secret" {
		t.Fatalf("auth header: %q", gotAuth)
	}
}

func TestEmbed_CountMismatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(embedResp{Data: []struct {
			Embedding []float32 `json:"embedding"`
		}{{Embedding: []float32{0.1}}}})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "", "")
	if _, err := c.Embed(context.Background(), []string{"a", "b"}); err == nil {
		t.Fatal("expected mismatch error")
	}
}

func TestNilClient_EmbedErrors(t *testing.T) {
	var c *Client
	if _, err := c.Embed(context.Background(), []string{"x"}); err == nil {
		t.Fatal("expected error from nil client")
	}
}

func TestEmbed_EmptyInput(t *testing.T) {
	c := NewClient("http://x", "", "")
	vecs, err := c.Embed(context.Background(), nil)
	if err != nil || vecs != nil {
		t.Fatalf("unexpected: %v %v", vecs, err)
	}
}
