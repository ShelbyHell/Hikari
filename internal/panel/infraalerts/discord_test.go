package infraalerts

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestParseDiscordWebhook(t *testing.T) {
	for _, c := range []struct {
		in string
		ok bool
	}{
		{"https://discord.com/api/webhooks/123456/AbC-d_e", true},
		{" https://discordapp.com/api/v10/webhooks/1/x ", true},
		{"https://canary.discord.com/api/webhooks/1/x?thread_id=99", true},
		{"https://Discord.COM/api/webhooks/1/x", true},
		{"https://DISCORD.com.evil.example/api/webhooks/1/x", false},
		{"http://discord.com/api/webhooks/1/x", false},
		{"https://discord.com.evil.example/api/webhooks/1/x", false},
		{"https://evil.example/api/webhooks/1/x", false},
		{"https://user@discord.com/api/webhooks/1/x", false},
		{"https://discord.com:8443/api/webhooks/1/x", false},
		{"https://discord.com/api/webhooks/1/x/slack", false},
		{"https://discord.com/api/webhooks/1/x?wait=true", false},
		{"https://discord.com/api/webhooks/1/x?thread_id=a", false},
		{"https://discord.com/api/webhooks/1/x#f", false},
		{"", false},
	} {
		got, err := ParseDiscordWebhook(c.in)
		want := strings.Replace(strings.TrimSpace(c.in), "Discord.COM", "discord.com", 1) // the host in lower case
		if (err == nil) != c.ok || c.ok && got != want {
			t.Errorf("ParseDiscordWebhook(%q) = %q, %v; want ok=%v", c.in, got, err, c.ok)
		}
	}
}

func TestDiscordText(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"🔴 <b>n&lt;1&gt; &amp; m</b> — down", "🔴 **n<1> & m** — down"},
		{"<b>a*b_c</b> ~x~ `y` |z|", `**a\*b\_c** \~x\~ ` + "\\`y\\`" + ` \|z\|`},
		{"<i>i</i> <code>a*b</code> <a href=\"https://x.example/?a=1&amp;b=2\">link</a>", "*i* `a*b` [link](<https://x.example/?a=1&b=2>)"},
		{"<tg-spoiler>s</tg-spoiler> 1 < 2", "s 1 < 2"},
		{"@everyone", "@everyone"},
	} {
		if got := DiscordText(c.in); got != c.want {
			t.Errorf("DiscordText(%q) = %q, want %q", c.in, got, c.want)
		}
	}
	if got := []rune(DiscordText(strings.Repeat("я", 3000))); len(got) != discordMax {
		t.Errorf("a long text is %d characters", len(got))
	}
}

func TestDiscordClientPost(t *testing.T) {
	var body map[string]any
	status := http.StatusNoContent
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_, _ = io.WriteString(w, `{"name":"alerts","token":"secret"}`)
			return
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if status == http.StatusTooManyRequests {
			w.Header().Set("Retry-After", "2.5")
			w.WriteHeader(status)
			_, _ = io.WriteString(w, `{"message":"You are being rate limited.","retry_after":2.5}`)
			return
		}
		w.WriteHeader(status)
	}))
	t.Cleanup(srv.Close)
	c := &DiscordClient{HTTP: srv.Client()}
	hook := srv.URL + "/api/webhooks/1/tok3n"
	if err := c.Post(context.Background(), hook, "<b>x</b> @everyone"); err != nil {
		t.Fatal(err)
	}
	if body["content"] != "**x** @everyone" {
		t.Fatalf("content %v", body["content"])
	}
	if am, _ := body["allowed_mentions"].(map[string]any); am == nil || len(am["parse"].([]any)) != 0 {
		t.Fatalf("allowed_mentions %v", body["allowed_mentions"])
	}
	status = http.StatusTooManyRequests
	err := c.Post(context.Background(), hook, "x")
	var de *DiscordError
	if !errors.As(err, &de) || de.Code != 429 || de.RetryAfter != 2500*time.Millisecond {
		t.Fatalf("429: %v", err)
	}
	if name, err := c.Check(context.Background(), hook+"?thread_id=5"); err != nil || name != "alerts" {
		t.Fatalf("Check = %q, %v", name, err)
	}
	srv.Close()
	err = c.Post(context.Background(), hook, "x")
	if err == nil || strings.Contains(err.Error(), "tok3n") {
		t.Fatalf("an error with the address in it: %v", err)
	}
}
