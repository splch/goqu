package ibm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const defaultIAMURL = "https://iam.cloud.ibm.com/identity/token"

// tokenProvider manages IBM Cloud IAM token lifecycle.
type tokenProvider struct {
	apiKey     string
	iamURL     string
	mu         sync.Mutex
	token      string
	expiry     time.Time
	httpClient *http.Client
}

func newTokenProvider(apiKey string) *tokenProvider {
	return &tokenProvider{
		apiKey:     apiKey,
		iamURL:     defaultIAMURL,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

// iamTokenResponse is the JSON response from the IAM token endpoint.
type iamTokenResponse struct {
	AccessToken string `json:"access_token"`
	Expiration  int64  `json:"expiration"` // Unix timestamp
	ExpiresIn   int    `json:"expires_in"` // seconds
}

// getToken returns a valid bearer token, refreshing if necessary.
// It caches the token and refreshes when within 60 seconds of expiry.
func (tp *tokenProvider) getToken(ctx context.Context) (string, error) {
	tp.mu.Lock()
	defer tp.mu.Unlock()

	// Return cached token if still valid (with 60s buffer).
	if tp.token != "" && time.Now().Before(tp.expiry.Add(-60*time.Second)) {
		return tp.token, nil
	}

	// Exchange API key for IAM bearer token.
	form := url.Values{
		"grant_type": {"urn:ibm:params:oauth:grant-type:apikey"},
		"apikey":     {tp.apiKey},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tp.iamURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("ibm: create IAM request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := tp.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("ibm: IAM token request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("ibm: read IAM response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ibm: IAM token exchange failed (%d): %s", resp.StatusCode, string(body))
	}

	var tokenResp iamTokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", fmt.Errorf("ibm: unmarshal IAM response: %w", err)
	}

	if tokenResp.AccessToken == "" {
		return "", fmt.Errorf("ibm: IAM returned empty access token")
	}

	tp.token = tokenResp.AccessToken
	if tokenResp.Expiration > 0 {
		tp.expiry = time.Unix(tokenResp.Expiration, 0)
	} else if tokenResp.ExpiresIn > 0 {
		tp.expiry = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
	} else {
		// Default: assume 1 hour validity.
		tp.expiry = time.Now().Add(time.Hour)
	}

	return tp.token, nil
}
