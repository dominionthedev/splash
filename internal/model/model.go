// Package model provides the LLM abstraction.
// The rest of splash only depends on the Model interface — swap freely.
package model

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

// Turn is one message in a conversation.
type Turn struct {
	Role    string // "user" | "assistant"
	Content string
}

// Model is the LLM interface the agent uses.
// The system prompt and conversation history are passed each call.
type Model interface {
	Chat(ctx context.Context, system string, history []Turn, input string) (string, error)
}

// ── Ollama ────────────────────────────────────────────────────────────────

type ollamaModel struct {
	baseURL string
	name    string
	client  *http.Client
}

// NewOllama returns a Model backed by an Ollama-compatible server.
// Reads OLLAMA_HOST or OLLACLOUD_HOST from environment.
func NewOllama(name string) Model {
	base := firstEnv("OLLAMA_HOST", "OLLACLOUD_HOST")
	if base == "" {
		base = "http://localhost:11434"
	}
	if name == "" {
		name = "llama3.2"
	}
	return &ollamaModel{
		baseURL: base,
		name:    name,
		client:  &http.Client{Timeout: 120 * time.Second},
	}
}

type ollamaReq struct {
	Model    string       `json:"model"`
	Messages []ollamaMsg  `json:"messages"`
	Stream   bool         `json:"stream"`
}

type ollamaMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ollamaResp struct {
	Message ollamaMsg `json:"message"`
}

func (m *ollamaModel) Chat(ctx context.Context, system string, history []Turn, input string) (string, error) {
	msgs := []ollamaMsg{{Role: "system", Content: system}}
	for _, t := range history {
		msgs = append(msgs, ollamaMsg{Role: t.Role, Content: t.Content})
	}
	msgs = append(msgs, ollamaMsg{Role: "user", Content: input})

	body, _ := json.Marshal(ollamaReq{
		Model:    m.name,
		Messages: msgs,
		Stream:   false,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		m.baseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("model: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := m.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("model: request failed: %w", err)
	}
	defer resp.Body.Close()

	var out ollamaResp
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("model: decode: %w", err)
	}
	return strings.TrimSpace(out.Message.Content), nil
}

func firstEnv(keys ...string) string {
	for _, k := range keys {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	return ""
}
