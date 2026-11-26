package app

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"mikan/internal/panel/domain"
	"mikan/internal/panel/subs"
)

func TestSubscriptionCheckOverHTTP(t *testing.T) {
	k := newKeyHarness(t)
	ctx := t.Context()
	resp, body := k.do(http.MethodPatch, k.api+"/settings", map[string]any{"device_binding": true, "device_require_hwid": true}, k.csrf)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("require HWID: %d %s", resp.StatusCode, body)
	}
	tariffs, err := k.st.Q.ListTariffs(ctx)
	if err != nil || len(tariffs) < 2 {
		t.Fatalf("seeded tariffs: %+v (%v)", tariffs, err)
	}
	clock := func() time.Time { return k.now }
	u, err := domain.NewUsers(k.st, domain.NewPool(k.st, clock), noChanges{}, clock).Create(ctx, domain.CreateInput{Name: "Subscriber", TariffID: tariffs[1].ID})
	if err != nil {
		t.Fatal(err)
	}
	slot, err := k.st.Q.GetSlot(ctx, u.SlotID.Int64)
	if err != nil {
		t.Fatal(err)
	}
	path := "/users/" + idOf(u.ID) + "/subscription-check"
	read := func(resp *http.Response, raw []byte, tuicReason string) {
		t.Helper()
		var r subs.SubscriptionCheck
		if resp.StatusCode != http.StatusOK || resp.Header.Get("Cache-Control") != "no-store" || json.Unmarshal(raw, &r) != nil {
			t.Fatalf("diagnostics: %d %s", resp.StatusCode, raw)
		}
		for _, secret := range []string{u.SubToken, slot.Uuid, slot.Secret, "203.0.113.10", "://"} {
			if secret != "" && strings.Contains(string(raw), secret) {
				t.Fatalf("credentials or endpoint in report: %s", raw)
			}
		}
		for _, n := range r.Nodes {
			for _, in := range n.Inbounds {
				if in.Protocol == "tuic" {
					if in.Reason != tuicReason || (tuicReason == "" && in.ProfileName == "") {
						t.Fatalf("TUIC: %+v, want reason %q", in, tuicReason)
					}
					return
				}
			}
		}
		t.Fatalf("missing seeded TUIC: %s", raw)
	}
	// No X-Hwid: this is not a real subscription fetch, even with device binding enforced.
	resp, body = k.do(http.MethodGet, k.api+path+"?app=happ", nil, nil)
	read(resp, body, "unsupported_app")
	resp, body = k.do(http.MethodGet, k.api+path+"?app=clash", nil, nil)
	read(resp, body, "")
	resp, body = k.asKey(k.read, http.MethodGet, path+"?app=clash&mihomo_version=v1.19.34", nil)
	read(resp, body, "")
	for _, query := range []string{"", "?app=other", "?app=clash&mihomo_version=1.19"} {
		if resp, raw := k.do(http.MethodGet, k.api+path+query, nil, nil); resp.StatusCode != http.StatusUnprocessableEntity {
			t.Errorf("invalid query %q: %d %s", query, resp.StatusCode, raw)
		}
	}
	if bound, err := k.st.Q.ListBoundDeviceSlots(ctx, u.ID); err != nil || len(bound) != 0 {
		t.Fatalf("the check registered devices: %+v (%v)", bound, err)
	}
	if fetches, err := k.st.Q.ListSubFetchesSince(ctx, 0); err != nil || len(fetches) != 0 {
		t.Fatalf("the check was recorded as a fetch: %+v (%v)", fetches, err)
	}
	if after, err := k.st.Q.GetUser(ctx, u.ID); err != nil || after.SubToken != u.SubToken || after.SlotID != u.SlotID || after.UpdatedAt != u.UpdatedAt {
		t.Fatalf("the check changed the user: %v", err)
	}
}
