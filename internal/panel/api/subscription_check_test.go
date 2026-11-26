package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/danielgtaylor/huma/v2/adapters/humago"
)

func TestSubscriptionCheckQuery(t *testing.T) {
	mux := http.NewServeMux()
	h := &handlers{api: humago.New(mux, Config("test"))}
	h.registerSubscriptionCheck([]string{"users"})
	// The service is not wired in this test. Only accepted queries reach that check.
	for _, tc := range []struct {
		query string
		want  int
	}{
		{"?app=happ", http.StatusServiceUnavailable},
		{"?app=clash", http.StatusServiceUnavailable},
		{"?app=clash&mihomo_version=v1.19.34", http.StatusServiceUnavailable},
		{"", http.StatusUnprocessableEntity},
		{"?app=other", http.StatusUnprocessableEntity},
		{"?app=clash&mihomo_version=1.19", http.StatusUnprocessableEntity},
	} {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/users/1/subscription-check"+tc.query, nil))
		if w.Code != tc.want {
			t.Errorf("%s: got %d, want %d: %s", tc.query, w.Code, tc.want, w.Body)
		}
	}
}
