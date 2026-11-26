package subs

import (
	"database/sql"
	"encoding/json"
	"strings"
	"testing"

	"mikan/internal/panel/domain"
	"mikan/internal/panel/presets"
	"mikan/internal/panel/store/db"
)

func checkInbound(t *testing.T, id, nodeID int64, preset, name string) db.Inbound {
	t.Helper()
	p, ok := presets.Get(preset)
	if !ok {
		t.Fatalf("unknown preset %q", preset)
	}
	cfg, err := presets.NewConfig(preset, "www.example.com:443")
	if err != nil {
		t.Fatal(err)
	}
	return db.Inbound{ID: id, NodeID: nodeID, Preset: preset, Enabled: 1, Port: p.Port, Name: name, DisplayName: name, Config: cfg}
}

func TestSubscriptionCheckReasons(t *testing.T) {
	vision := checkInbound(t, 1, 1, "vless_reality_vision", "Vision")
	tuic := checkInbound(t, 2, 1, "tuic_v5", "TUIC")
	trust := checkInbound(t, 3, 2, "trusttunnel", "TrustTunnel")
	variant := func(id int64, name string) db.Inbound {
		x := vision
		x.ID, x.Name, x.DisplayName = id, name, name
		return x
	}
	disabled, spent, excluded, denied := variant(4, "disabled"), variant(5, "spent"), variant(6, "excluded"), variant(7, "denied")
	disabled.Enabled = 0
	spent.PoolID = sql.NullInt64{Int64: 41, Valid: true}
	excluded.PoolID = sql.NullInt64{Int64: 42, Valid: true}
	offNode, badConfig, badPort := variant(8, "off-node"), variant(9, "bad-config"), variant(10, "bad-port")
	offNode.NodeID = 3
	badConfig.Config = "type: [unclosed"
	badPort.Port = "not-a-port"
	shared := checkInbound(t, 11, 1, "shadowsocks_2022", "Shared")
	xhttp := checkInbound(t, 12, 1, "vless_reality_xhttp", "XHTTP")
	nodes := []db.Node{{ID: 1, Enabled: 1}, {ID: 2, Enabled: 1}, {ID: 3}, {ID: 4, Enabled: 1}}
	prof := Profile{Nodes: []Node{{ID: 1}, {ID: 2}, {ID: 4}}}
	ins := profileInbounds{
		all:          []db.Inbound{vision, tuic, trust, disabled, spent, excluded, denied, offNode, badConfig, badPort, shared, xhttp},
		nodes:        map[int64]bool{1: true, 2: true, 4: true},
		allowed:      []int64{1, 2, 3, 4, 5, 6, 8, 9, 10, 11, 12},
		blockedPools: map[int64]string{41: domain.PoolBlockedTraffic, 42: domain.PoolBlockedPlan},
	}
	for _, tc := range []struct {
		ua, state, tuic, trust, shared, xhttp, minimum string
	}{
		{"Happ/3.6.0", domain.StateActive, reasonUnsupportedApp, reasonUnsupportedApp, "", "", ""},
		{"clash-verge/2.2.3", domain.StateActive, "", reasonCoreUnknown, "", "", "1.19.21"},
		{"mihomo/1.19.20", domain.StateActive, "", reasonCoreTooOld, "", reasonCoreTooOld, "1.19.21"},
		{"mihomo/1.19.34", domain.StateExpired, "", "", reasonSharedNeedsAccess, "", ""},
	} {
		t.Run(tc.ua, func(t *testing.T) {
			r, err := checkProfile(prof, ins, nodes, tc.state, DetectApp(tc.ua), "")
			if err != nil || len(r.Nodes) != 4 || r.State != tc.state {
				t.Fatalf("report: %+v (%v)", r, err)
			}
			second := ""
			if tc.trust != "" {
				second = "no_matching_inbounds"
			}
			for i, want := range []string{"", second, "node_disabled", "no_inbounds"} {
				if got := r.Nodes[i].Reason; got != want || r.Nodes[i].Inbounds == nil {
					t.Errorf("node %d: got %+v, want reason %q and a non-nil array", i+1, r.Nodes[i], want)
				}
			}
			want := map[int64]string{
				1: "", 2: tc.tuic, 3: tc.trust, 4: reasonInboundDisabled, 5: domain.PoolBlockedTraffic,
				6: domain.PoolBlockedPlan, 7: reasonNotAllowed, 8: "node_disabled", 9: reasonInvalidConfig,
				10: reasonRenderFailed, 11: tc.shared, 12: tc.xhttp,
			}
			for _, n := range r.Nodes {
				for _, row := range n.Inbounds {
					if reason, ok := want[row.ID]; !ok || reason != row.Reason || (row.Reason == "") != (row.ProfileName == row.Name) {
						t.Errorf("unexpected result for %s: %+v, want reason %q", row.Name, row, reason)
					}
					if row.ID == trust.ID && row.MinimumMihomo != tc.minimum || row.ID == xhttp.ID && tc.xhttp != "" && row.MinimumMihomo != "1.19.22" {
						t.Errorf("wrong core requirement: %+v", row)
					}
					delete(want, row.ID)
				}
			}
			if len(want) != 0 {
				t.Errorf("missing inbounds: %v", want)
			}
		})
	}
}

func TestSubscriptionCheckTemplateAndExternalServers(t *testing.T) {
	in := checkInbound(t, 1, 1, "vless_reality_vision", "Vision")
	ins := profileInbounds{all: []db.Inbound{in}, nodes: map[int64]bool{1: true}}
	nodes := []db.Node{{ID: 1, Enabled: 1}}
	const externalSecret = "external-secret-not-in-report"
	const host = "203.0.113.10"
	prof := Profile{
		Nodes: []Node{{ID: 1, Endpoint: Endpoint{Host: host}}},
		External: []ExternalServer{
			{Group: "External", Name: "Clash only", Mihomo: map[string]any{"name": "Clash only", "password": externalSecret}},
			{Group: "External", Name: "Supports Happ", Mihomo: map[string]any{"name": "Supports Happ"}, Link: "trojan://" + externalSecret + "@" + host + ":443"},
		},
	}
	src := "proxies:\n  - name: Vision\n    type: direct\nrules:\n  - MATCH,DIRECT\n"
	for _, tc := range []struct {
		ua, template, reason string
		external             int
		fallback             bool
	}{
		{"mihomo/1.19.34", src, reasonTemplateName, 2, false},
		{"mihomo/1.19.34", "rules:\n  - MATCH,missing-group\n", "", 2, true},
		{"Happ", src, "", 1, false},
	} {
		r, err := checkProfile(prof, ins, nodes, domain.StateActive, DetectApp(tc.ua), tc.template)
		if err != nil || r.ExternalIncluded != tc.external || r.TemplateFallback != tc.fallback || r.Nodes[0].Inbounds[0].Reason != tc.reason {
			t.Fatalf("%s: report %+v (%v)", tc.ua, r, err)
		}
		raw, err := json.Marshal(r)
		if err != nil {
			t.Fatal(err)
		}
		for _, secret := range []string{externalSecret, host, previewSlot.Uuid, "trojan://", "uuid", "password"} {
			if strings.Contains(string(raw), secret) {
				t.Errorf("unexpected data in report: %s", raw)
			}
		}
	}
}
