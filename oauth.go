package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	OAuthClientID = "9e5f94bc-e8a4-4e73-b8be-63364c29d753"
	OAuthScope    = "https://graph.microsoft.com/Mail.Read"
	refreshSkew   = 5 * time.Minute
	refreshRetry  = 30 * time.Second
)

type OAuthManager struct {
	refreshToken     string
	refreshTokenFile string
	accessToken      string
	expiresAt        time.Time
	mu               sync.Mutex
}

func NewOAuthManager(refreshToken, refreshTokenFile string) *OAuthManager {
	return &OAuthManager{
		refreshToken:     refreshToken,
		refreshTokenFile: refreshTokenFile,
	}
}

func (m *OAuthManager) StartAutoRefresh() {
	go func() {
		for {
			_, expiresAt, err := m.RefreshAccessToken()
			if err != nil {
				log.Printf("[MAIL] OAuth refresh timer error: %v", err)
				time.Sleep(refreshRetry)
				continue
			}

			wait := time.Until(expiresAt.Add(-refreshSkew))
			if wait < refreshRetry {
				wait = refreshRetry
			}
			log.Printf("[MAIL] OAuth token refreshed; next refresh in %s", wait.Round(time.Second))
			time.Sleep(wait)
		}
	}()
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

	return m.refreshLocked()
}

func (m *OAuthManager) RefreshAccessToken() (string, time.Time, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.refreshToken == "" {
		return "", time.Time{}, fmt.Errorf("no refresh token provided")
	}

	token, err := m.refreshLocked()
	if err != nil {
		return "", time.Time{}, err
	}
	return token, m.expiresAt, nil
}

func (m *OAuthManager) refreshLocked() (string, error) {
	form := url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {OAuthClientID},
		"refresh_token": {m.refreshToken},
		"scope":         {OAuthScope},
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
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", fmt.Errorf("failed to parse token response: %v", err)
	}
	if tokenResp.AccessToken == "" {
		return "", fmt.Errorf("token refresh returned empty access token")
	}

	m.accessToken = tokenResp.AccessToken
	m.expiresAt = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
	if tokenResp.RefreshToken != "" && tokenResp.RefreshToken != m.refreshToken {
		m.refreshToken = tokenResp.RefreshToken
		if err := writeRefreshTokenFile(m.refreshTokenFile, tokenResp.RefreshToken); err != nil {
			return "", fmt.Errorf("failed to persist refresh token: %v", err)
		}
	}

	return m.accessToken, nil
}

func writeRefreshTokenFile(path string, token string) error {
	if path == "" || token == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(token+"\n"), 0o600)
}
