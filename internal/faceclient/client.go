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

// FaceQuality contains face quality metrics.
type FaceQuality struct {
	Score     float64 `json:"score"`
	Blur      float64 `json:"blur"`
	PoseYaw   float64 `json:"pose_yaw"`
	PosePitch float64 `json:"pose_pitch"`
	PoseRoll  float64 `json:"pose_roll"`
	FaceSize  int     `json:"face_size"`
	IsFrontal bool    `json:"is_frontal"`
}

// EmbedResult contains the face embedding and detection confidence.
type EmbedResult struct {
	Embedding     []float32
	Score         float64
	FacesDetected int
	Quality       *FaceQuality
}

// CompareResult contains face comparison results.
type CompareResult struct {
	Similarity float64
	Match      bool
	Threshold  float64
	Quality1   *FaceQuality
	Quality2   *FaceQuality
}

// EnrollResult contains face enrollment response.
type EnrollResult struct {
	UserID  string
	Success bool
	Quality *FaceQuality
	Message string
}

// SearchMatch represents a face match from gallery search.
type SearchMatch struct {
	UserID     string  `json:"user_id"`
	Similarity float64 `json:"similarity"`
	Name       string  `json:"name,omitempty"`
}

// SearchResult contains 1:N search results.
type SearchResult struct {
	Matches       []SearchMatch
	FacesDetected int
	Quality       *FaceQuality
}

// VerifyResult contains 1:1 verification result.
type VerifyResult struct {
	UserID     string
	Verified   bool
	Similarity float64
	Threshold  float64
	Quality    *FaceQuality
}

// LivenessResult contains anti-spoofing check result.
type LivenessResult struct {
	IsLive     bool
	Confidence float64
	Checks     map[string]interface{}
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
			Quality: &FaceQuality{
				Score:     0.85,
				Blur:      0.1,
				PoseYaw:   5.0,
				PosePitch: 3.0,
				PoseRoll:  1.0,
				FaceSize:  40000,
				IsFrontal: true,
			},
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
		Embedding     []float32    `json:"embedding"`
		Score         float64      `json:"score"`
		FacesDetected int          `json:"faces_detected"`
		Quality       *FaceQuality `json:"quality"`
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
		Quality:       out.Quality,
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

// Enroll enrolls a face into the recognition gallery for 1:N search.
func (c *Client) Enroll(ctx context.Context, userID, imageURL, name string, metadata map[string]interface{}) (*EnrollResult, error) {
	if c.Skip {
		return &EnrollResult{
			UserID:  userID,
			Success: true,
			Quality: &FaceQuality{Score: 0.85, IsFrontal: true},
			Message: "Face enrolled (mock)",
		}, nil
	}

	payload := map[string]interface{}{
		"user_id":   userID,
		"image_url": imageURL,
	}
	if name != "" {
		payload["name"] = name
	}
	if metadata != nil {
		payload["metadata"] = metadata
	}

	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/enroll", bytes.NewReader(body))
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
		UserID  string       `json:"user_id"`
		Success bool         `json:"success"`
		Quality *FaceQuality `json:"quality"`
		Message string       `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &EnrollResult{
		UserID:  out.UserID,
		Success: out.Success,
		Quality: out.Quality,
		Message: out.Message,
	}, nil
}

// Search performs 1:N face identification against enrolled gallery.
func (c *Client) Search(ctx context.Context, imageURL string, topK int, threshold float64) (*SearchResult, error) {
	if c.Skip {
		return &SearchResult{
			Matches:       []SearchMatch{{UserID: "mock-user", Similarity: 0.92, Name: "Mock User"}},
			FacesDetected: 1,
			Quality:       &FaceQuality{Score: 0.85, IsFrontal: true},
		}, nil
	}

	payload := map[string]interface{}{
		"image_url": imageURL,
		"top_k":     topK,
	}
	if threshold > 0 {
		payload["threshold"] = threshold
	}

	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/search", bytes.NewReader(body))
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
		Matches       []SearchMatch `json:"matches"`
		FacesDetected int           `json:"faces_detected"`
		Quality       *FaceQuality  `json:"quality"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &SearchResult{
		Matches:       out.Matches,
		FacesDetected: out.FacesDetected,
		Quality:       out.Quality,
	}, nil
}

// Verify performs 1:1 face verification against a specific enrolled user.
func (c *Client) Verify(ctx context.Context, userID, imageURL string) (*VerifyResult, error) {
	if c.Skip {
		return &VerifyResult{
			UserID:     userID,
			Verified:   true,
			Similarity: 0.92,
			Threshold:  0.45,
			Quality:    &FaceQuality{Score: 0.85, IsFrontal: true},
		}, nil
	}

	body, _ := json.Marshal(map[string]string{
		"user_id":   userID,
		"image_url": imageURL,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/verify", bytes.NewReader(body))
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
		UserID     string       `json:"user_id"`
		Verified   bool         `json:"verified"`
		Similarity float64      `json:"similarity"`
		Threshold  float64      `json:"threshold"`
		Quality    *FaceQuality `json:"quality"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &VerifyResult{
		UserID:     out.UserID,
		Verified:   out.Verified,
		Similarity: out.Similarity,
		Threshold:  out.Threshold,
		Quality:    out.Quality,
	}, nil
}

// Liveness checks if the face image is from a live person (anti-spoofing).
func (c *Client) Liveness(ctx context.Context, imageURL string) (*LivenessResult, error) {
	if c.Skip {
		return &LivenessResult{
			IsLive:     true,
			Confidence: 0.85,
			Checks:     map[string]interface{}{"mock": true},
		}, nil
	}

	body, _ := json.Marshal(map[string]string{"image_url": imageURL})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/liveness", bytes.NewReader(body))
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
		IsLive     bool                   `json:"is_live"`
		Confidence float64                `json:"confidence"`
		Checks     map[string]interface{} `json:"checks"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &LivenessResult{
		IsLive:     out.IsLive,
		Confidence: out.Confidence,
		Checks:     out.Checks,
	}, nil
}
