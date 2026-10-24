package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"mikan/internal/panel/settings"
)

func TestSubDomainRoutes(t *testing.T) {
	admin := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "admin") })
	sub := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "sub") })
	s := New(admin, sub)
	s.SetPaths(settings.Paths{Admin: "admin-path", Sub: "sub-path"})
	s.SetDomain("panel.example.com")
	s.SetSubDomain("sub.example.com")
	s.SetSNIDomain("sni.example.com")

	get := func(host, path string) (int, string) {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Host = host
		rec := httptest.NewRecorder()
		s.ServeHTTP(rec, req)
		return rec.Code, rec.Body.String()
	}

	// 1. Admin path on regular host -> OK
	if code, body := get("panel.example.com", "/admin-path/"); code != http.StatusOK || body != "admin" {
		t.Fatalf("admin on panel domain: %d %q", code, body)
	}

	// 2. Subscription path on regular host -> OK
	if code, body := get("panel.example.com", "/sub-path/"); code != http.StatusOK || body != "sub" {
		t.Fatalf("sub on panel domain: %d %q", code, body)
	}

	// 3. Subscription path on sub_domain -> OK
	if code, body := get("sub.example.com", "/sub-path/"); code != http.StatusOK || body != "sub" {
		t.Fatalf("sub on sub_domain: %d %q", code, body)
	}
	if code, body := get("sub.example.com:443", "/sub-path/"); code != http.StatusOK || body != "sub" {
		t.Fatalf("sub on sub_domain:443: %d %q", code, body)
	}

	// 4. Admin path on sub_domain -> 404 (Admin panel is hidden on subscription domain)
	if code, _ := get("sub.example.com", "/admin-path/"); code != http.StatusNotFound {
		t.Fatalf("admin on sub_domain: want 404, got %d", code)
	}
	if code, _ := get("sub.example.com:8443", "/admin-path/"); code != http.StatusNotFound {
		t.Fatalf("admin on sub_domain:8443: want 404, got %d", code)
	}

	// 5. Admin and sub paths on sni_domain -> 404 (Both hidden on SNI domain)
	if code, _ := get("sni.example.com", "/admin-path/"); code != http.StatusNotFound {
		t.Fatalf("admin on sni_domain: want 404, got %d", code)
	}
	if code, _ := get("sni.example.com", "/sub-path/"); code != http.StatusNotFound {
		t.Fatalf("sub on sni_domain: want 404, got %d", code)
	}
	if code, _ := get("sni.example.com:443", "/sub-path/"); code != http.StatusNotFound {
		t.Fatalf("sub on sni_domain:443: want 404, got %d", code)
	}

	// 6. If sni_domain is mistakenly set to the same as panel domain, it never blocks admin on panel domain
	s.SetSNIDomain("panel.example.com")
	if code, body := get("panel.example.com", "/admin-path/"); code != http.StatusOK || body != "admin" {
		t.Fatalf("admin on panel domain when sni_domain matches panel: %d %q", code, body)
	}
}
