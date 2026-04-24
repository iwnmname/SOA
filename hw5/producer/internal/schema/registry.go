package schema

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"go.uber.org/zap"
)

type Registry struct {
	url    string
	logger *zap.Logger
	client *http.Client
}

type registerRequest struct {
	SchemaType string `json:"schemaType"`
	Schema     string `json:"schema"`
}

type registerResponse struct {
	ID int `json:"id"`
}

func NewRegistry(url string, logger *zap.Logger) *Registry {
	return &Registry{
		url:    url,
		logger: logger,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (r *Registry) WaitForReady(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := r.client.Get(r.url + "/subjects")
		if err == nil && resp.StatusCode == 200 {
			resp.Body.Close()
			r.logger.Info("schema registry is ready")
			return nil
		}
		if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("schema registry not ready after %v", timeout)
}

func (r *Registry) RegisterProto(subject, protoFilePath string) (int, error) {
	content, err := os.ReadFile(protoFilePath)
	if err != nil {
		return 0, fmt.Errorf("read proto file: %w", err)
	}

	reqBody := registerRequest{
		SchemaType: "PROTOBUF",
		Schema:     string(content),
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return 0, fmt.Errorf("marshal: %w", err)
	}

	url := fmt.Sprintf("%s/subjects/%s/versions", r.url, subject)
	resp, err := r.client.Post(url, "application/vnd.schemaregistry.v1+json", bytes.NewReader(body))
	if err != nil {
		return 0, fmt.Errorf("post: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return 0, fmt.Errorf("registry returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result registerResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return 0, fmt.Errorf("unmarshal: %w", err)
	}

	r.logger.Info("schema registered",
		zap.String("subject", subject),
		zap.Int("id", result.ID),
	)

	return result.ID, nil
}
