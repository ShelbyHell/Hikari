package api

import (
	cryptohmac "crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"mikan/internal/panel/tgate"
)

func TestParseCookie(t *testing.T) {
	raw := "foo=bar; gh_oauth_state=secret123; other=val"
	if got := parseCookie(raw, "gh_oauth_state"); got != "secret123" {
		t.Fatalf("parseCookie expected 'secret123', got %q", got)
	}
	if got := parseCookie(raw, "missing"); got != "" {
		t.Fatalf("parseCookie expected empty string, got %q", got)
	}
	if got := parseCookie("", "any"); got != "" {
		t.Fatalf("parseCookie on empty header expected empty string, got %q", got)
	}
}

func TestTelegramWidgetVerification(t *testing.T) {
	const botToken = "123456789:ABCdefGHIjklMNOpqrSTUvwxYZ"
	now := time.Now()
	authDate := strconv.FormatInt(now.Unix(), 10)

	params := map[string]string{
		"id":         "999888777",
		"first_name": "Antigravity",
		"username":   "antigravity",
		"auth_date":  authDate,
	}

	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	lines := make([]string, len(keys))
	for i, k := range keys {
		lines[i] = k + "=" + params[k]
	}

	secret := sha256.Sum256([]byte(botToken))
	mac := cryptohmac.New(sha256.New, secret[:])
	mac.Write([]byte(strings.Join(lines, "\n")))
	params["hash"] = hex.EncodeToString(mac.Sum(nil))

	u, err := tgate.CheckTelegramWidget(botToken, params, now)
	if err != nil {
		t.Fatalf("CheckTelegramWidget failed: %v", err)
	}
	if u.ID != 999888777 || u.Username != "antigravity" {
		t.Fatalf("unexpected widget user: %+v", u)
	}
}

func TestGitHubAuthorizeURLFormat(t *testing.T) {
	clientID := "github_client_123"
	state := "random_state_abc"
	callbackURL := "https://panel.example.com/api/v1/auth/github/callback"

	authURL := "https://github.com/login/oauth/authorize?client_id=" + url.QueryEscape(clientID) +
		"&scope=read:user&state=" + url.QueryEscape(state) +
		"&redirect_uri=" + url.QueryEscape(callbackURL)

	parsed, err := url.Parse(authURL)
	if err != nil {
		t.Fatalf("url parse error: %v", err)
	}
	q := parsed.Query()
	if q.Get("client_id") != clientID {
		t.Errorf("wrong client_id: %s", q.Get("client_id"))
	}
	if q.Get("state") != state {
		t.Errorf("wrong state: %s", q.Get("state"))
	}
	if q.Get("redirect_uri") != callbackURL {
		t.Errorf("wrong redirect_uri: %s", q.Get("redirect_uri"))
	}
}
