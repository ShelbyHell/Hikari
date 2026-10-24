package settings

import (
	"context"
	"testing"
	"time"

	"mikan/internal/panel/store/db"
	"mikan/internal/panel/store/storetest"
)

// A cached Settings keeps a value for its ttl: what another process writes (the CLI, here
// straight into the table) waits for it, what this process writes is seen at once.
func TestCachedSettings(t *testing.T) {
	ctx := context.Background()
	st, err := storetest.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	plain, cached := New(st.Q), Cached(st.Q, time.Hour)
	if err := Set(ctx, plain, KeyDefaultLang, "ru"); err != nil {
		t.Fatal(err)
	}
	writtenAt.Store(time.Now().Add(-writeGrace).UnixNano()) // as if written a while ago

	lang := func() string {
		t.Helper()
		v, err := cached.Lang(ctx)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	if lang() != "ru" {
		t.Fatal("first read")
	}
	if err := st.Q.SetSetting(ctx, db.SetSettingParams{Key: KeyDefaultLang, Value: `"en"`}); err != nil {
		t.Fatal(err)
	}
	if lang() != "ru" {
		t.Fatal("a value written by another process must wait for the ttl")
	}
	if err := Set(ctx, plain, KeyDefaultLang, "en"); err != nil {
		t.Fatal(err)
	}
	if lang() != "en" {
		t.Fatal("a value written by this process must be seen at once")
	}
	// Read within writeGrace of the write: not kept, the next read asks again.
	if err := st.Q.SetSetting(ctx, db.SetSettingParams{Key: KeyDefaultLang, Value: `"ru"`}); err != nil {
		t.Fatal(err)
	}
	if lang() != "ru" {
		t.Fatal("a read right after a write must not be kept: its transaction may not have committed")
	}
	if v, _, _ := Get[string](ctx, plain, KeyDefaultLang); v != "ru" {
		t.Fatal("a plain Settings reads the table every time")
	}
}

func TestEndpointURL(t *testing.T) {
	for _, c := range []struct {
		ep   Endpoint
		want string
	}{
		{Endpoint{Host: "sub.example.com", Port: 443}, "https://sub.example.com"},
		{Endpoint{Host: "sub.example.com", Port: 8443}, "https://sub.example.com:8443"},
		{Endpoint{Host: "2001:db8::1", Port: 443}, "https://[2001:db8::1]"},
		{Endpoint{Host: "2001:db8::1", Port: 2053}, "https://[2001:db8::1]:2053"},
		{Endpoint{Host: "", Port: 443}, ""},
		{Endpoint{Host: "sub.example.com", Port: 0}, ""},
	} {
		if got := c.ep.URL(); got != c.want {
			t.Errorf("URL(%+v) = %q, want %q", c.ep, got, c.want)
		}
	}
}

func TestSubEndpoint(t *testing.T) {
	ctx := context.Background()
	st, err := storetest.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	s := New(st.Q)

	// Default: public_host + panel_port
	if err := Set(ctx, s, KeyPublicHost, "1.2.3.4"); err != nil {
		t.Fatal(err)
	}
	if err := Set(ctx, s, KeyPanelPort, 8443); err != nil {
		t.Fatal(err)
	}
	ep, err := s.SubEndpoint(ctx)
	if err != nil || ep.Host != "1.2.3.4" || ep.Port != 8443 {
		t.Fatalf("default: %+v %v", ep, err)
	}

	// Domain set
	if err := Set(ctx, s, KeyDomain, "panel.example.com"); err != nil {
		t.Fatal(err)
	}
	ep, err = s.SubEndpoint(ctx)
	if err != nil || ep.Host != "panel.example.com" || ep.Port != 8443 {
		t.Fatalf("domain: %+v %v", ep, err)
	}

	// SubPort set
	if err := Set(ctx, s, KeySubPort, 2053); err != nil {
		t.Fatal(err)
	}
	ep, err = s.SubEndpoint(ctx)
	if err != nil || ep.Host != "panel.example.com" || ep.Port != 2053 {
		t.Fatalf("sub_port: %+v %v", ep, err)
	}

	// SubDomain set: overrides domain
	if err := Set(ctx, s, KeySubDomain, "sub.example.com"); err != nil {
		t.Fatal(err)
	}
	ep, err = s.SubEndpoint(ctx)
	if err != nil || ep.Host != "sub.example.com" || ep.Port != 2053 {
		t.Fatalf("sub_domain: %+v %v", ep, err)
	}
	if u := ep.URL(); u != "https://sub.example.com:2053" {
		t.Fatalf("sub URL: %q", u)
	}

	// Reset SubDomain -> falls back to domain
	if err := Set(ctx, s, KeySubDomain, ""); err != nil {
		t.Fatal(err)
	}
	ep, err = s.SubEndpoint(ctx)
	if err != nil || ep.Host != "panel.example.com" || ep.Port != 2053 {
		t.Fatalf("sub_domain reset: %+v %v", ep, err)
	}

	// SNIDomain set & reset
	if err := Set(ctx, s, KeySNIDomain, "sni.example.com"); err != nil {
		t.Fatal(err)
	}
	if v, err := s.String(ctx, KeySNIDomain); err != nil || v != "sni.example.com" {
		t.Fatalf("sni_domain: %q %v", v, err)
	}
	if err := Set(ctx, s, KeySNIDomain, ""); err != nil {
		t.Fatal(err)
	}
	if v, err := s.String(ctx, KeySNIDomain); err != nil || v != "" {
		t.Fatalf("sni_domain reset: %q %v", v, err)
	}
}

