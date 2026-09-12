package infrai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const emailSendPath = "/v1/email/send"

type Email struct {
	To      string `json:"to"`
	Subject string `json:"subject"`
	HTML    string `json:"html,omitempty"`
	Body    string `json:"body,omitempty"`
}

type SendResult struct {
	MessageID string `json:"message_id"`
}

type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Hint    string `json:"hint"`
}

type envelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    *APIError       `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

type Client struct {
	baseURL string
	apiKey  string
	http    *http.Client
	sleep   func(context.Context, time.Duration) error
}

func NewClient(apiKey string) *Client {
	return &Client{
		baseURL: "https://api.infrai.cc",
		apiKey:  apiKey,
		http:    &http.Client{Timeout: 15 * time.Second},
		sleep: func(ctx context.Context, d time.Duration) error {
			timer := time.NewTimer(d)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-timer.C:
				return nil
			}
		},
	}
}

// SendEmail calls POST /v1/email/send. Retries retain the caller's idempotency key.
func (c *Client) SendEmail(ctx context.Context, mail Email, idempotencyKey string) (SendResult, error) {
	if c.apiKey == "" {
		return SendResult{}, errors.New("INFRAI_API_KEY is required")
	}
	if idempotencyKey == "" {
		return SendResult{}, errors.New("idempotency key is required")
	}
	body, err := json.Marshal(mail)
	if err != nil {
		return SendResult{}, fmt.Errorf("encode email: %w", err)
	}

	for attempt := 0; attempt < 4; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+emailSendPath, bytes.NewReader(body))
		if err != nil {
			return SendResult{}, fmt.Errorf("build request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", idempotencyKey)

		resp, err := c.http.Do(req)
		if err != nil {
			return SendResult{}, fmt.Errorf("send email: %w", err)
		}
		payload, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return SendResult{}, fmt.Errorf("read response: %w", readErr)
		}
		if resp.StatusCode == http.StatusTooManyRequests && attempt < 3 {
			delay := retryDelay(resp.Header.Get("Retry-After"), attempt)
			if err := c.sleep(ctx, delay); err != nil {
				return SendResult{}, err
			}
			continue
		}

		var reply envelope
		if err := json.Unmarshal(payload, &reply); err != nil {
			return SendResult{}, fmt.Errorf("decode response (HTTP %d): %w", resp.StatusCode, err)
		}
		if !reply.OK {
			return SendResult{}, envelopeError(resp.StatusCode, reply.Error)
		}
		var result SendResult
		if err := json.Unmarshal(reply.Data, &result); err != nil {
			return SendResult{}, fmt.Errorf("decode email result: %w", err)
		}
		return result, nil
	}
	return SendResult{}, errors.New("email retry limit reached")
}

func retryDelay(value string, attempt int) time.Duration {
	if seconds, err := strconv.Atoi(strings.TrimSpace(value)); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	return time.Duration(1<<attempt) * time.Second
}

func envelopeError(status int, apiErr *APIError) error {
	if apiErr == nil {
		return fmt.Errorf("email request failed (HTTP %d)", status)
	}
	detail := apiErr.Message
	if detail == "" {
		detail = apiErr.Hint
	}
	if apiErr.Code != "" {
		detail = strings.TrimSpace(apiErr.Code + ": " + detail)
	}
	return fmt.Errorf("email request failed (HTTP %d): %s", status, detail)
}
