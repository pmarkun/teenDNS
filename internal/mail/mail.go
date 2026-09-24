// Package mail sends transactional email through Resend's HTTP API. It has no
// SDK dependency: a single POST covers everything the gateway needs.
package mail

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

// Sender delivers a single HTML email. Implementations must be safe for
// concurrent use.
type Sender interface {
	Send(to, subject, html string) error
}

// ResendClient sends email through https://api.resend.com. With an empty
// APIKey it logs instead of sending, so local labs and tests never need a
// real credential.
type ResendClient struct {
	APIKey string
	From   string

	// Endpoint overrides the Resend API URL. Empty means the default.
	Endpoint string

	httpClient *http.Client
}

// NewResendClient builds a client for the given API key and From address.
// An empty apiKey is valid and puts the client in log-only mode.
func NewResendClient(apiKey, from string) *ResendClient {
	return &ResendClient{
		APIKey:     apiKey,
		From:       from,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

type resendRequest struct {
	From    string   `json:"from"`
	To      []string `json:"to"`
	Subject string   `json:"subject"`
	HTML    string   `json:"html"`
}

func (c *ResendClient) Send(to, subject, html string) error {
	if strings.TrimSpace(c.APIKey) == "" {
		log.Printf("[mail] log-only mode: enviaria para %s: %s", to, subject)
		return nil
	}

	body, err := json.Marshal(resendRequest{From: c.From, To: []string{to}, Subject: subject, HTML: html})
	if err != nil {
		return fmt.Errorf("encode email: %w", err)
	}

	endpoint := c.Endpoint
	if endpoint == "" {
		endpoint = "https://api.resend.com/emails"
	}
	request, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+c.APIKey)

	client := c.httpClient
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("send email: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode >= 300 {
		responseBody, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return fmt.Errorf("resend responded %d: %s", response.StatusCode, strings.TrimSpace(string(responseBody)))
	}
	return nil
}
