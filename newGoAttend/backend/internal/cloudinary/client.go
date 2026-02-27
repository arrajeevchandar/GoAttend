package cloudinary

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"time"
)

// Client uploads images to Cloudinary via unsigned upload (no signature needed).
type Client struct {
	cloudName string
	apiKey    string
	apiSecret string
	uploadURL string
}

type UploadResult struct {
	SecureURL string `json:"secure_url"`
	PublicID  string `json:"public_id"`
}

// New parses a CLOUDINARY_URL and returns a Client.
// Format: cloudinary://API_KEY:API_SECRET@CLOUD_NAME
func New(cloudinaryURL string) (*Client, error) {
	if cloudinaryURL == "" {
		return nil, fmt.Errorf("CLOUDINARY_URL is empty")
	}

	u, err := url.Parse(cloudinaryURL)
	if err != nil {
		return nil, fmt.Errorf("parse cloudinary url: %w", err)
	}

	apiKey := u.User.Username()
	apiSecret, _ := u.User.Password()
	cloudName := u.Host

	return &Client{
		cloudName: cloudName,
		apiKey:    apiKey,
		apiSecret: apiSecret,
		uploadURL: fmt.Sprintf("https://api.cloudinary.com/v1_1/%s/image/upload", cloudName),
	}, nil
}

// Upload uploads image bytes to Cloudinary and returns the secure URL.
func (c *Client) Upload(fileData io.Reader, filename string, folder string) (*UploadResult, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	// File field
	fw, err := w.CreateFormFile("file", filename)
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(fw, fileData); err != nil {
		return nil, err
	}

	// Upload preset or signed params
	w.WriteField("upload_preset", "goattend") // Create an unsigned upload preset named "goattend" in Cloudinary
	if folder != "" {
		w.WriteField("folder", folder)
	}
	w.Close()

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Post(c.uploadURL, w.FormDataContentType(), &buf)
	if err != nil {
		return nil, fmt.Errorf("cloudinary upload: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("cloudinary error %d: %s", resp.StatusCode, string(body))
	}

	var result UploadResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode cloudinary response: %w", err)
	}

	return &result, nil
}
