package main

import (
	"os"
	"strings"
)

const defaultRefreshTokenFile = "tokens/outlook_refresh_token"

type Config struct {
	RefreshToken     string
	RefreshTokenFile string
	ListenAddr       string
}

func LoadConfig() *Config {
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
		RefreshToken:     refreshToken,
		RefreshTokenFile: refreshTokenFile,
		ListenAddr:       listenAddr,
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
