package main

import (
	"os"
	"strings"
)

const defaultRefreshTokenFile = "tokens/outlook_refresh_token"

type Config struct {
	PrimaryEmail     string
	RefreshToken     string
	RefreshTokenFile string
	ListenAddr       string
	AliasStartNum    int
}

func LoadConfig() *Config {
	email := os.Getenv("OUTLOOK_EMAIL")
	if email == "" {
		email = "test@outlook.com" // Placeholder
	}

	refreshToken := os.Getenv("OUTLOOK_REFRESH_TOKEN")
	refreshTokenFile := os.Getenv("OUTLOOK_REFRESH_TOKEN_FILE")
	if refreshTokenFile == "" {
		refreshTokenFile = defaultRefreshTokenFile
	}
	if refreshToken == "" {
		refreshToken = readRefreshTokenFile(refreshTokenFile)
	}

	listenAddr := os.Getenv("LISTEN_ADDR")
	if listenAddr == "" {
		listenAddr = ":50053"
	}

	return &Config{
		PrimaryEmail:     email,
		RefreshToken:     refreshToken,
		RefreshTokenFile: refreshTokenFile,
		ListenAddr:       listenAddr,
		AliasStartNum:    1000,
	}
}

func readRefreshTokenFile(path string) string {
	if path == "" {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}
