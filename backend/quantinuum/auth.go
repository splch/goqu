package quantinuum

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

const defaultLoginURL = "https://qapi.quantinuum.com/v1/login"

// tokenProvider manages Quantinuum JWT token lifecycle.
type tokenProvider struct {
	email      string
	password   string
	loginURL   string
	mu         sync.Mutex
	token      string
	expiry     time.Time
	httpClient *http.Client
}

func newTokenProvider(email, password string) *tokenProvider {
	return &tokenProvider{
		email:      email,
		password:   password,
		loginURL:   defaultLoginURL,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

// getToken returns a valid JWT token, refreshing if necessary.
// It caches the token and refreshes when within 60 seconds of expiry.
func (tp *tokenProvider) getToken(ctx context.Context) (string, error) {
	tp.mu.Lock()
	defer tp.mu.Unlock()

	// Return cached token if still valid (with 60s buffer).
	if tp.token != "" && time.Now().Before(tp.expiry.Add(-60*time.Second)) {
		return tp.token, nil
	}

	body, err := json.Marshal(loginRequest{ // #nosec G117 -- password is sent intentionally to the auth endpoint
		Email:    tp.email,
		Password: tp.password,
	})
	if err != nil {
		return "", fmt.Errorf("quantinuum: marshal login request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tp.loginURL, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("quantinuum: create login request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := tp.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("quantinuum: login request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("quantinuum: read login response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("quantinuum: login failed (%d): %s", resp.StatusCode, string(respBody))
	}

	var loginResp loginResponse
	if err := json.Unmarshal(respBody, &loginResp); err != nil {
		return "", fmt.Errorf("quantinuum: unmarshal login response: %w", err)
	}

	if loginResp.IDToken == "" {
		return "", fmt.Errorf("quantinuum: login returned empty id-token")
	}

	tp.token = loginResp.IDToken
	tp.expiry = extractJWTExpiry(loginResp.IDToken)

	return tp.token, nil
}

// extractJWTExpiry decodes the exp claim from a JWT token.
// Falls back to 55 minutes from now if decoding fails.
func extractJWTExpiry(token string) time.Time {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return time.Now().Add(55 * time.Minute)
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return time.Now().Add(55 * time.Minute)
	}

	var claims struct {
		Exp int64 `json:"exp"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil || claims.Exp == 0 {
		return time.Now().Add(55 * time.Minute)
	}

	return time.Unix(claims.Exp, 0)
}
