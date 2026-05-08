package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
)

const graphMessagesURL = "https://graph.microsoft.com/v1.0/me/mailFolders/inbox/messages"

var otpPattern = regexp.MustCompile(`(^|[^0-9])([0-9]{6})([^0-9]|$)`)

type Waiter struct {
	EmailAddress   string
	SubjectKeyword string
	ResponseChan   chan string
	CreatedAt      time.Time
}

type MailWatcher struct {
	cfg      *Config
	accMgr   *AccountManager
	oauthMgr *OAuthManager
	waiters  map[string]*Waiter
	mu       sync.Mutex
}

func NewMailWatcher(cfg *Config, accMgr *AccountManager) *MailWatcher {
	return &MailWatcher{
		cfg:      cfg,
		accMgr:   accMgr,
		oauthMgr: NewOAuthManager(cfg.RefreshToken, cfg.OAuthScope),
		waiters:  make(map[string]*Waiter),
	}
}

func (w *MailWatcher) AddWaiter(emailAddr, subjectKeyword string, respChan chan string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	key := normalizeEmail(emailAddr)
	w.waiters[key] = &Waiter{
		EmailAddress:   emailAddr,
		SubjectKeyword: subjectKeyword,
		ResponseChan:   respChan,
		CreatedAt:      time.Now(),
	}
	log.Printf("[MAIL] Added waiter for %s (subject: %s)", redactEmail(emailAddr), subjectKeyword)
}

func (w *MailWatcher) RemoveWaiter(emailAddr string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	delete(w.waiters, normalizeEmail(emailAddr))
}

func (w *MailWatcher) getWaiters() map[string]*Waiter {
	w.mu.Lock()
	defer w.mu.Unlock()

	now := time.Now()
	for k, v := range w.waiters {
		if now.Sub(v.CreatedAt) > 10*time.Minute {
			log.Printf("[MAIL] Removing stale waiter for %s", redactEmail(v.EmailAddress))
			delete(w.waiters, k)
		}
	}

	copy := make(map[string]*Waiter)
	for k, v := range w.waiters {
		copy[k] = v
	}
	return copy
}

func (w *MailWatcher) Start() {
	go func() {
		for {
			w.poll()
			time.Sleep(5 * time.Second)
		}
	}()
}

func (w *MailWatcher) poll() {
	waiters := w.getWaiters()
	if len(waiters) == 0 {
		return
	}

	_, refreshToken := w.accMgr.GetCredentials()
	if refreshToken == "" {
		log.Println("[MAIL] No refresh token configured, skipping poll")
		return
	}

	accessToken, err := w.oauthMgr.GetAccessToken()
	if err != nil {
		log.Printf("[MAIL] OAuth token error: %v", err)
		return
	}

	messages, err := fetchRecentMessages(accessToken)
	if err != nil {
		log.Printf("[MAIL] Graph fetch error: %v", err)
		return
	}

	delivered := make(map[string]bool)
	for _, msg := range messages {
		waiter, recipient := matchWaiter(msg, waiters)
		if waiter == nil {
			continue
		}
		waiterKey := normalizeEmail(waiter.EmailAddress)
		if delivered[waiterKey] {
			continue
		}
		if !containsFold(msg.Subject, waiter.SubjectKeyword) {
			continue
		}

		otp := extractOTP(msg.BodyPreview + "\n" + msg.Body.Content)
		if otp == "" {
			continue
		}

		if recipient == "" {
			recipient = waiter.EmailAddress
		}
		log.Printf("[MAIL] Found OTP for %s", redactEmail(recipient))
		select {
		case waiter.ResponseChan <- otp:
		default:
		}
		delivered[waiterKey] = true
		w.RemoveWaiter(waiter.EmailAddress)
	}
}

type graphMessageList struct {
	Value []graphMessage `json:"value"`
}

type graphMessage struct {
	Subject                string                `json:"subject"`
	BodyPreview            string                `json:"bodyPreview"`
	Body                   graphBody             `json:"body"`
	ToRecipients           []graphRecipient      `json:"toRecipients"`
	CcRecipients           []graphRecipient      `json:"ccRecipients"`
	BccRecipients          []graphRecipient      `json:"bccRecipients"`
	InternetMessageHeaders []graphInternetHeader `json:"internetMessageHeaders"`
}

type graphBody struct {
	Content string `json:"content"`
}

type graphRecipient struct {
	EmailAddress graphEmailAddress `json:"emailAddress"`
}

type graphEmailAddress struct {
	Address string `json:"address"`
}

type graphInternetHeader struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

func fetchRecentMessages(accessToken string) ([]graphMessage, error) {
	u, err := url.Parse(graphMessagesURL)
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("$top", "25")
	q.Set("$orderby", "receivedDateTime desc")
	q.Set("$select", "subject,bodyPreview,body,toRecipients,ccRecipients,bccRecipients,internetMessageHeaders,receivedDateTime")
	u.RawQuery = q.Encode()

	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("status=%d body=%s", resp.StatusCode, string(body[:min(len(body), 500)]))
	}

	var out graphMessageList
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	return out.Value, nil
}

func matchWaiter(msg graphMessage, waiters map[string]*Waiter) (*Waiter, string) {
	for _, addr := range messageAddresses(msg) {
		key := normalizeEmail(addr)
		if waiter, ok := waiters[key]; ok {
			return waiter, addr
		}
	}
	if len(waiters) == 1 {
		return nil, ""
	}
	return nil, ""
}

func messageAddresses(msg graphMessage) []string {
	var out []string
	addRecipients := func(recipients []graphRecipient) {
		for _, r := range recipients {
			if r.EmailAddress.Address != "" {
				out = append(out, r.EmailAddress.Address)
			}
		}
	}
	addRecipients(msg.ToRecipients)
	addRecipients(msg.CcRecipients)
	addRecipients(msg.BccRecipients)

	for _, h := range msg.InternetMessageHeaders {
		name := strings.ToLower(h.Name)
		if name == "to" || name == "delivered-to" || name == "x-original-to" || name == "envelope-to" {
			out = append(out, extractEmails(h.Value)...)
		}
	}
	return out
}

func extractEmails(s string) []string {
	fields := strings.FieldsFunc(s, func(r rune) bool {
		return r == '<' || r == '>' || r == ',' || r == ';' || r == '"' || r == '\'' || r == '(' || r == ')'
	})
	var emails []string
	for _, f := range fields {
		f = strings.TrimSpace(f)
		if strings.Contains(f, "@") {
			emails = append(emails, f)
		}
	}
	return emails
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func redactEmail(email string) string {
	email = strings.TrimSpace(email)
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return "***"
	}
	local := parts[0]
	if len(local) > 2 {
		local = local[:2] + "***"
	} else {
		local = "***"
	}
	return local + "@" + parts[1]
}

func containsFold(s, substr string) bool {
	if substr == "" {
		return true
	}
	haystack := strings.ToLower(s)
	needle := strings.ToLower(substr)
	if needle == "openai" {
		return strings.Contains(haystack, "openai") || strings.Contains(haystack, "chatgpt")
	}
	return strings.Contains(haystack, needle)
}

func extractOTP(body string) string {
	match := otpPattern.FindStringSubmatch(body)
	if len(match) >= 3 {
		return match[2]
	}
	return ""
}
