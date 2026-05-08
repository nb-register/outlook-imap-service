package main

import (
	"os"
)

type Config struct {
	PrimaryEmail  string
	RefreshToken  string
	OAuthScope    string
	ListenAddr    string
	AliasStartNum int
}

func LoadConfig() *Config {
	email := os.Getenv("OUTLOOK_EMAIL")
	if email == "" {
		email = "test@outlook.com" // Placeholder
	}

	refreshToken := os.Getenv("OUTLOOK_REFRESH_TOKEN")

	oauthScope := os.Getenv("OUTLOOK_AUTH_SCOPE")
	if oauthScope == "" {
		oauthScope = "https://graph.microsoft.com/Mail.Read"
	}

	listenAddr := os.Getenv("LISTEN_ADDR")
	if listenAddr == "" {
		listenAddr = ":50053"
	}

	return &Config{
		PrimaryEmail:  email,
		RefreshToken:  refreshToken,
		OAuthScope:    oauthScope,
		ListenAddr:    listenAddr,
		AliasStartNum: 1000,
	}
}
