package ionq

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/splch/qgo/observe"
)

const (
	defaultBaseURL = "https://api.ionq.co/v0.4"
	userAgent      = "qgo/0.1.0"
)

type httpClient struct {
	base    *http.Client
	baseURL string
	apiKey  string
	backend string // backend name for observability (e.g., "ionq.simulator")
}

func newHTTPClient(apiKey, baseURL string, base *http.Client) *httpClient {
	if base == nil {
		base = &http.Client{Timeout: 30 * time.Second}
	}
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return &httpClient{base: base, baseURL: baseURL, apiKey: apiKey}
}

// do executes an HTTP request with JSON marshaling and retry logic.
func (c *httpClient) do(ctx context.Context, method, path string, body, resp any) error {
	var attempt int
	for {
		err := c.doOnce(ctx, method, path, body, resp)
		if err == nil {
			return nil
		}
		apiErr, ok := err.(*APIError)
		if !ok || !apiErr.Retryable() || attempt >= 3 {
			return err
		}
		delay := apiErr.retryDelay(attempt)
		attempt++
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (c *httpClient) doOnce(ctx context.Context, method, path string, body, resp any) error {
	hooks := observe.FromContext(ctx)
	var httpDone func(int, error)
	if hooks != nil && hooks.WrapHTTP != nil {
		ctx, httpDone = hooks.WrapHTTP(ctx, observe.HTTPInfo{
			Method:  method,
			Path:    path,
			Backend: c.backend,
		})
	}

	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			if httpDone != nil {
				httpDone(0, err)
			}
			return fmt.Errorf("ionq: marshal request: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
	if err != nil {
		if httpDone != nil {
			httpDone(0, err)
		}
		return fmt.Errorf("ionq: create request: %w", err)
	}
	req.Header.Set("Authorization", "apiKey "+c.apiKey)
	req.Header.Set("User-Agent", userAgent)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	httpResp, err := c.base.Do(req)
	if err != nil {
		if httpDone != nil {
			httpDone(0, err)
		}
		return fmt.Errorf("ionq: http request: %w", err)
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		if httpDone != nil {
			httpDone(httpResp.StatusCode, err)
		}
		return fmt.Errorf("ionq: read response: %w", err)
	}

	if httpResp.StatusCode >= 400 {
		apiErr := &APIError{
			StatusCode: httpResp.StatusCode,
			RetryAfter: httpResp.Header.Get("Retry-After"),
		}
		var parsed ionqAPIError
		if json.Unmarshal(respBody, &parsed) == nil {
			apiErr.Code = parsed.Err
			apiErr.Message = parsed.Message
		} else {
			apiErr.Message = string(respBody)
		}
		if httpDone != nil {
			httpDone(httpResp.StatusCode, apiErr)
		}
		return apiErr
	}

	if resp != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, resp); err != nil {
			if httpDone != nil {
				httpDone(httpResp.StatusCode, err)
			}
			return fmt.Errorf("ionq: unmarshal response: %w", err)
		}
	}

	if httpDone != nil {
		httpDone(httpResp.StatusCode, nil)
	}
	return nil
}

// APIError represents an error response from the IonQ API.
type APIError struct {
	StatusCode int
	Code       string
	Message    string
	RetryAfter string // from Retry-After header
}

func (e *APIError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("ionq: %d %s: %s", e.StatusCode, e.Code, e.Message)
	}
	return fmt.Sprintf("ionq: %d: %s", e.StatusCode, e.Message)
}

// Retryable reports whether this error is transient and the request can be retried.
func (e *APIError) Retryable() bool {
	return e.StatusCode == 429 || e.StatusCode >= 500
}

func (e *APIError) retryDelay(attempt int) time.Duration {
	if e.RetryAfter != "" {
		if secs, err := strconv.Atoi(e.RetryAfter); err == nil {
			return time.Duration(secs) * time.Second
		}
	}
	// Exponential backoff: 1s, 2s, 4s
	return time.Duration(1<<attempt) * time.Second
}
