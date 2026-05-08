package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const OAuthClientID = "9e5f94bc-e8a4-4e73-b8be-63364c29d753"

type OAuthManager struct {
	refreshToken string
	scope        string
	accessToken  string
	expiresAt    time.Time
	mu           sync.Mutex
}

func NewOAuthManager(refreshToken, scope string) *OAuthManager {
	return &OAuthManager{
		refreshToken: refreshToken,
		scope:        scope,
	}
}

func (m *OAuthManager) GetAccessToken() (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.refreshToken == "" {
		return "", fmt.Errorf("no refresh token provided")
	}

	// Add 1-minute buffer for expiration
	if m.accessToken != "" && time.Now().Before(m.expiresAt.Add(-1*time.Minute)) {
		return m.accessToken, nil
	}

	form := url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {OAuthClientID},
		"refresh_token": {m.refreshToken},
	}
	if m.scope != "" {
		form.Set("scope", m.scope)
	}

	tokenReq, _ := http.NewRequest("POST", "https://login.microsoftonline.com/common/oauth2/v2.0/token", strings.NewReader(form.Encode()))
	tokenReq.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(tokenReq)
	if err != nil {
		return "", fmt.Errorf("failed to refresh token: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token refresh failed: %s", string(body))
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", fmt.Errorf("failed to parse token response: %v", err)
	}

	m.accessToken = tokenResp.AccessToken
	m.expiresAt = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)

	return m.accessToken, nil
}
