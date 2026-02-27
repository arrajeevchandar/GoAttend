package faceclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

type RegisterResult struct {
	Status    string `json:"status"`
	StudentID string `json:"student_id"`
}

type RecognizeResult struct {
	Matched   bool    `json:"matched"`
	StudentID string  `json:"student_id,omitempty"`
	Distance  float64 `json:"distance,omitempty"`
}

func New(baseURL string) *Client {
	return &Client{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Client) Register(studentID string, photoData io.Reader, filename string) (*RegisterResult, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	w.WriteField("student_id", studentID)
	fw, err := w.CreateFormFile("photo", filename)
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(fw, photoData); err != nil {
		return nil, err
	}
	w.Close()

	resp, err := c.httpClient.Post(c.baseURL+"/register", w.FormDataContentType(), &buf)
	if err != nil {
		return nil, fmt.Errorf("face service register: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("face service error %d: %s", resp.StatusCode, string(body))
	}

	var result RegisterResult
	json.NewDecoder(resp.Body).Decode(&result)
	return &result, nil
}

func (c *Client) Recognize(photoData io.Reader, filename string) (*RecognizeResult, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, err := w.CreateFormFile("photo", filename)
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(fw, photoData); err != nil {
		return nil, err
	}
	w.Close()

	resp, err := c.httpClient.Post(c.baseURL+"/recognize", w.FormDataContentType(), &buf)
	if err != nil {
		return nil, fmt.Errorf("face service recognize: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("face service error %d: %s", resp.StatusCode, string(body))
	}

	var result RecognizeResult
	json.NewDecoder(resp.Body).Decode(&result)
	return &result, nil
}
