package cloudinary

import (
	"bytes"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Client uploads images to Cloudinary using their REST API.
type Client struct {
	CloudName string
	APIKey    string
	APISecret string
	Folder    string
	HTTP      *http.Client
}

// New creates a Cloudinary client.
func New(cloudName, apiKey, apiSecret, folder string) *Client {
	return &Client{
		CloudName: cloudName,
		APIKey:    apiKey,
		APISecret: apiSecret,
		Folder:    folder,
		HTTP:      &http.Client{Timeout: 30 * time.Second},
	}
}

// UploadResult holds the response from Cloudinary after a successful upload.
type UploadResult struct {
	PublicID  string `json:"public_id"`
	SecureURL string `json:"secure_url"`
	URL       string `json:"url"`
	Format    string `json:"format"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	Bytes     int    `json:"bytes"`
}

// UploadBase64 uploads a base64 data URL image to Cloudinary.
// data should be a full data URL like "data:image/jpeg;base64,..."
// or just raw base64 — both are accepted.
func (c *Client) UploadBase64(data string) (*UploadResult, error) {
	// Cloudinary accepts data URIs directly via the "file" param
	params := map[string]string{
		"timestamp": strconv.FormatInt(time.Now().Unix(), 10),
		"api_key":   c.APIKey,
	}
	if c.Folder != "" {
		params["folder"] = c.Folder
	}

	params["signature"] = c.sign(params)

	// Build multipart form
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	for k, v := range params {
		_ = w.WriteField(k, v)
	}
	_ = w.WriteField("file", data)
	w.Close()

	url := fmt.Sprintf("https://api.cloudinary.com/v1_1/%s/image/upload", c.CloudName)
	req, err := http.NewRequest(http.MethodPost, url, &buf)
	if err != nil {
		return nil, fmt.Errorf("cloudinary: create request failed: %w", err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cloudinary: request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("cloudinary: upload failed (%d): %s", resp.StatusCode, string(body))
	}

	var result UploadResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("cloudinary: decode response failed: %w", err)
	}
	return &result, nil
}

// UploadBytes uploads raw image bytes to Cloudinary.
func (c *Client) UploadBytes(data []byte, filename string) (*UploadResult, error) {
	params := map[string]string{
		"timestamp": strconv.FormatInt(time.Now().Unix(), 10),
		"api_key":   c.APIKey,
	}
	if c.Folder != "" {
		params["folder"] = c.Folder
	}
	params["signature"] = c.sign(params)

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	for k, v := range params {
		_ = w.WriteField(k, v)
	}

	part, err := w.CreateFormFile("file", filename)
	if err != nil {
		return nil, fmt.Errorf("cloudinary: create form file failed: %w", err)
	}
	if _, err := io.Copy(part, bytes.NewReader(data)); err != nil {
		return nil, fmt.Errorf("cloudinary: write file failed: %w", err)
	}
	w.Close()

	url := fmt.Sprintf("https://api.cloudinary.com/v1_1/%s/image/upload", c.CloudName)
	req, err := http.NewRequest(http.MethodPost, url, &buf)
	if err != nil {
		return nil, fmt.Errorf("cloudinary: create request failed: %w", err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cloudinary: request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("cloudinary: upload failed (%d): %s", resp.StatusCode, string(body))
	}

	var result UploadResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("cloudinary: decode response failed: %w", err)
	}
	return &result, nil
}

// sign computes the Cloudinary API signature from the given params.
// api_key and file are excluded from the signature per Cloudinary spec.
func (c *Client) sign(params map[string]string) string {
	excludeKeys := map[string]bool{"api_key": true, "file": true, "resource_type": true}

	pairs := make([]string, 0, len(params))
	for k, v := range params {
		if !excludeKeys[k] && v != "" {
			pairs = append(pairs, k+"="+v)
		}
	}
	sort.Strings(pairs)

	payload := strings.Join(pairs, "&") + c.APISecret
	h := sha1.New()
	h.Write([]byte(payload))
	return fmt.Sprintf("%x", h.Sum(nil))
}
