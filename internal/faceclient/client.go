package faceclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// EmbedResult contains the face embedding and detection confidence.
type EmbedResult struct {
	Embedding     []float32
	Score         float64
	FacesDetected int
}

// CompareResult contains face comparison results.
type CompareResult struct {
	Similarity float64
	Match      bool
	Threshold  float64
}

// Client calls the face recognition microservice.
type Client struct {
	BaseURL string
	HTTP    *http.Client
	Skip    bool
}

// New creates a client with configurable timeout.
func New(baseURL string, skip bool) *Client {
	return &Client{
		BaseURL: baseURL,
		Skip:    skip,
		HTTP: &http.Client{
			Timeout: 30 * time.Second, // Face processing can take time
		},
	}
}

// Embed requests an embedding for an image URL (legacy method for compatibility).
func (c *Client) Embed(ctx context.Context, imageURL string) ([]float32, error) {
	result, err := c.EmbedWithScore(ctx, imageURL)
	if err != nil {
		return nil, err
	}
	return result.Embedding, nil
}

// EmbedWithScore requests an embedding and returns full result including score.
func (c *Client) EmbedWithScore(ctx context.Context, imageURL string) (*EmbedResult, error) {
	if c.Skip {
		return &EmbedResult{
			Embedding:     []float32{0.1, 0.2, 0.3},
			Score:         0.95,
			FacesDetected: 1,
		}, nil
	}
	if imageURL == "" {
		return nil, fmt.Errorf("image url required")
	}

	body, _ := json.Marshal(map[string]string{"image_url": imageURL})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/embed", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("face service request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("face service error %s: %s", resp.Status, string(bodyBytes))
	}

	var out struct {
		Embedding     []float32 `json:"embedding"`
		Score         float64   `json:"score"`
		FacesDetected int       `json:"faces_detected"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	if len(out.Embedding) == 0 {
		return nil, fmt.Errorf("no face detected in image")
	}

	return &EmbedResult{
		Embedding:     out.Embedding,
		Score:         out.Score,
		FacesDetected: out.FacesDetected,
	}, nil
}

// Compare compares two face images and returns similarity.
func (c *Client) Compare(ctx context.Context, imageURL1, imageURL2 string) (*CompareResult, error) {
	if c.Skip {
		return &CompareResult{
			Similarity: 0.85,
			Match:      true,
			Threshold:  0.5,
		}, nil
	}

	body, _ := json.Marshal(map[string]string{
		"image_url_1": imageURL1,
		"image_url_2": imageURL2,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/compare", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("face service request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("face service error %s: %s", resp.Status, string(bodyBytes))
	}

	var out CompareResult
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &out, nil
}

// Health checks if the face service is available.
func (c *Client) Health(ctx context.Context) error {
	if c.Skip {
		return nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/health", nil)
	if err != nil {
		return err
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("face service unavailable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("face service unhealthy: %s", resp.Status)
	}

	return nil
}
