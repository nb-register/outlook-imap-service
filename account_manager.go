package main

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
	"sync"
)

const aliasAlphabet = "abcdefghijklmnopqrstuvwxyz0123456789"

// AccountManager abstracts email alias generation and credentials
// This makes it easy to migrate to an account pool later.
type AccountManager struct {
	config       *Config
	mu           sync.Mutex
	aliasCounter int
}

func NewAccountManager(cfg *Config) *AccountManager {
	return &AccountManager{
		config:       cfg,
		aliasCounter: cfg.AliasStartNum,
	}
}

// GetNextEmail returns the next split email (e.g. primary+a1b2c3@outlook.com)
func (m *AccountManager) GetNextEmail() string {
	m.mu.Lock()
	defer m.mu.Unlock()

	primary := m.config.PrimaryEmail
	parts := strings.Split(primary, "@")
	if len(parts) != 2 {
		return primary // fallback
	}

	token, err := randomToken(6)
	if err != nil {
		alias := fmt.Sprintf("%s+%d@%s", parts[0], m.aliasCounter, parts[1])
		m.aliasCounter++
		return alias
	}
	alias := fmt.Sprintf("%s+%s@%s", parts[0], token, parts[1])
	m.aliasCounter++
	return alias
}

func randomToken(length int) (string, error) {
	var b strings.Builder
	b.Grow(length)
	max := big.NewInt(int64(len(aliasAlphabet)))

	for i := 0; i < length; i++ {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		b.WriteByte(aliasAlphabet[n.Int64()])
	}

	return b.String(), nil
}

func (m *AccountManager) GetCredentials() (string, string) {
	return m.config.PrimaryEmail, m.config.RefreshToken
}
