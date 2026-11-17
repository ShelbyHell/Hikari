package app

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"mikan/internal/nodetls"
	"mikan/internal/panel/domain"
	"mikan/internal/panel/settings"
)

// Cascades over HTTP: an inbound leaves through another node, a node's relay goes on
// to a third; self, loops, unknown nodes and the relay's port are refused.
func TestCascadeOverHTTP(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	if err := domain.Seed(ctx, h.st, h.now); err != nil {
		t.Fatal(err)
	}
	if err := settings.Set(ctx, settings.New(h.st.Q), settings.KeyPanelPort, 21355); err != nil {
		t.Fatal(err)
	}
	panel, _ := nodetls.Generate("mikan-panel", x509.ExtKeyUsageClientAuth, time.Now())
	b, _, err := domain.AddNode(ctx, h.st, panel, domain.NodeInput{Name: "B", Host: "198.51.100.20", APIPort: 40000}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	c, _, err := domain.AddNode(ctx, h.st, panel, domain.NodeInput{Name: "C", Host: "198.51.100.30", APIPort: 40000}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if resp, _ := h.login(password, ""); resp.StatusCode != http.StatusOK {
		t.Fatal("login")
	}
	csrf := map[string]string{"X-CSRF-Token": h.csrf}
	api := "/" + adminPath + "/api/v1"
	ins, _ := h.st.Q.ListInbounds(ctx)
	inbound := api + "/inbounds/" + strconv.FormatInt(ins[0].ID, 10)
	id := func(n int64) string { return strconv.FormatInt(n, 10) }

	for name, c := range map[string]struct {
		body map[string]any
		code string
	}{
		"itself":      {map[string]any{"outbound": "node", "exit_node_id": 1}, "exit_self"},
		"no node":     {map[string]any{"outbound": "node", "exit_node_id": 99}, "exit_not_found"},
		"no exit set": {map[string]any{"outbound": "node"}, "exit_not_found"},
	} {
		if resp, body := h.do(http.MethodPatch, inbound, c.body, csrf); resp.StatusCode != http.StatusUnprocessableEntity || !strings.Contains(string(body), c.code) {
			t.Fatalf("%s: %d %s", name, resp.StatusCode, body)
		}
	}
	resp, body := h.do(http.MethodPatch, inbound, map[string]any{"outbound": "node", "exit_node_id": b.ID}, csrf)
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(body), `"outbound":"node"`) || !strings.Contains(string(body), `"exit_node_id":`+id(b.ID)) {
		t.Fatalf("via B: %d %s", resp.StatusCode, body)
	}
	// B now has a relay with node 1 as its source.
	resp, body = h.do(http.MethodGet, api+"/nodes/"+id(b.ID)+"/cascade", nil, nil)
	var bv struct {
		Relay struct {
			Port    string `json:"port"`
			Sources []struct {
				NodeID int64 `json:"node_id"`
			} `json:"sources"`
			Outbound string `json:"outbound"`
		} `json:"relay"`
	}
	if resp.StatusCode != http.StatusOK || json.Unmarshal(body, &bv) != nil || bv.Relay.Port == "" || len(bv.Relay.Sources) != 1 || bv.Relay.Sources[0].NodeID != 1 || bv.Relay.Outbound != "direct" {
		t.Fatalf("B's relay: %d %s", resp.StatusCode, body)
	}
	if _, body := h.do(http.MethodGet, api+"/nodes/1/cascade", nil, nil); !strings.Contains(string(body), `"inbounds":["`+ins[0].Name+`"]`) {
		t.Fatalf("node 1 exits: %s", body)
	}
	// B → C, then C → B would loop.
	if resp, body := h.do(http.MethodPatch, api+"/nodes/"+id(b.ID)+"/cascade", map[string]any{"outbound": "node", "exit_node_id": c.ID}, csrf); resp.StatusCode != http.StatusOK {
		t.Fatalf("B → C: %d %s", resp.StatusCode, body)
	}
	if resp, body := h.do(http.MethodPatch, api+"/nodes/"+id(c.ID)+"/cascade", map[string]any{"outbound": "node", "exit_node_id": b.ID}, csrf); resp.StatusCode != http.StatusUnprocessableEntity || !strings.Contains(string(body), "exit_cycle") {
		t.Fatalf("loop: %d %s", resp.StatusCode, body)
	}
	if resp, _ := h.do(http.MethodPatch, api+"/nodes/"+id(c.ID)+"/cascade", map[string]any{"outbound": "warp"}, csrf); resp.StatusCode != http.StatusOK {
		t.Fatalf("C → WARP: %d", resp.StatusCode)
	}
	// An inbound on B may not take the relay's port.
	bIns, _ := h.st.Q.ListInbounds(ctx)
	for _, in := range bIns {
		if in.NodeID == b.ID && domain.InboundNetwork(in) == "tcp" {
			if resp, body := h.do(http.MethodPatch, api+"/inbounds/"+id(in.ID), map[string]any{"port": bv.Relay.Port}, csrf); resp.StatusCode != http.StatusConflict || !strings.Contains(string(body), "port_relay") {
				t.Fatalf("relay port: %d %s", resp.StatusCode, body)
			}
			break
		}
	}
	// Back to direct: the exit is gone.
	resp, body = h.do(http.MethodPatch, inbound, map[string]any{"outbound": "direct"}, csrf)
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(body), `"outbound":"direct"`) || strings.Contains(string(body), "exit_node_id") {
		t.Fatalf("direct: %d %s", resp.StatusCode, body)
	}
}

// Cascade rules over HTTP: part of a listener's traffic leaves through another node by
// a rule; the match is canonicalized, a foreign inbound, a bad match, self and loops are
// refused, and the order is the whole list of the node's rules.
func TestCascadeRulesOverHTTP(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	if err := domain.Seed(ctx, h.st, h.now); err != nil {
		t.Fatal(err)
	}
	if err := settings.Set(ctx, settings.New(h.st.Q), settings.KeyPanelPort, 21355); err != nil {
		t.Fatal(err)
	}
	panel, _ := nodetls.Generate("mikan-panel", x509.ExtKeyUsageClientAuth, time.Now())
	b, _, err := domain.AddNode(ctx, h.st, panel, domain.NodeInput{Name: "B", Host: "198.51.100.20", APIPort: 40000}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if resp, _ := h.login(password, ""); resp.StatusCode != http.StatusOK {
		t.Fatal("login")
	}
	csrf := map[string]string{"X-CSRF-Token": h.csrf}
	api := "/" + adminPath + "/api/v1"
	id := func(n int64) string { return strconv.FormatInt(n, 10) }
	rules := func(node int64) string { return api + "/nodes/" + id(node) + "/cascade/rules" }
	ins, _ := h.st.Q.ListInbounds(ctx)
	var own int64
	for _, in := range ins {
		if in.NodeID == 1 {
			own = in.ID
			break
		}
	}
	var bOwn int64
	for _, in := range ins {
		if in.NodeID == b.ID {
			bOwn = in.ID
			break
		}
	}

	// Refused: a match mihomo cannot parse, an inbound of another node, the node itself,
	// an unknown exit.
	for name, body := range map[string]map[string]any{
		"bad match":   {"match": "GEOSITE,youtube", "exit_node_id": b.ID},
		"bad match 2": {"match": "DOMAIN-SUFFIX,evil.com,DIRECT", "exit_node_id": b.ID},
		"no match":    {"exit_node_id": b.ID},
		"foreign inbound": {"match": "DOMAIN-SUFFIX,example.com", "inbound_id": bOwn, "exit_node_id": b.ID},
		"self":        {"match": "DOMAIN-SUFFIX,example.com", "exit_node_id": 1},
		"no exit":     {"match": "DOMAIN-SUFFIX,example.com", "exit_node_id": 99},
	} {
		if resp, rb := h.do(http.MethodPost, rules(1), body, csrf); resp.StatusCode != http.StatusUnprocessableEntity {
			t.Fatalf("%s: %d %s", name, resp.StatusCode, rb)
		}
	}
	// B's relay goes on to node 1, so a rule of node 1 to B would loop.
	if resp, rb := h.do(http.MethodPatch, api+"/nodes/"+id(b.ID)+"/cascade", map[string]any{"outbound": "node", "exit_node_id": 1}, csrf); resp.StatusCode != http.StatusOK {
		t.Fatalf("B → 1: %d %s", resp.StatusCode, rb)
	}
	if resp, rb := h.do(http.MethodPost, rules(1), map[string]any{"match": "DOMAIN-SUFFIX,example.com", "exit_node_id": b.ID}, csrf); resp.StatusCode != http.StatusUnprocessableEntity || !strings.Contains(string(rb), "exit_cycle") {
		t.Fatalf("loop: %d %s", resp.StatusCode, rb)
	}
	if resp, _ := h.do(http.MethodPatch, api+"/nodes/"+id(b.ID)+"/cascade", map[string]any{"outbound": "direct"}, csrf); resp.StatusCode != http.StatusOK {
		t.Fatal("B back to direct")
	}

	// Created: the match is canonical, the rule lands after the existing ones.
	resp, body := h.do(http.MethodPost, rules(1), map[string]any{"match": "domain-suffix,YouTube.COM", "inbound_id": own, "exit_node_id": b.ID}, csrf)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create: %d %s", resp.StatusCode, body)
	}
	var created struct {
		ID         int64  `json:"id"`
		Match      string `json:"match"`
		Inbound    string `json:"inbound"`
		ExitNodeID int64  `json:"exit_node_id"`
		ExitName   string `json:"exit_name"`
		Position   int64  `json:"position"`
	}
	// huma sends the Body field as the response body, unwrapped.
	if err := json.Unmarshal(body, &created); err != nil {
		t.Fatalf("body: %s", body)
	}
	if created.Match != "DOMAIN-SUFFIX,youtube.com" || created.ExitNodeID != b.ID || created.ExitName != "B" || created.Inbound == "" || created.Position != 1 {
		t.Fatalf("created rule: %s", body)
	}
	// A second one, any listener.
	resp, body = h.do(http.MethodPost, rules(1), map[string]any{"match": "DST-PORT,25-53", "exit_node_id": b.ID}, csrf)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create 2: %d %s", resp.StatusCode, body)
	}
	var second struct {
		ID       int64  `json:"id"`
		Match    string `json:"match"`
		Position int64  `json:"position"`
	}
	if err := json.Unmarshal(body, &second); err != nil || second.Match != "DST-PORT,25-53" || second.Position != 2 {
		t.Fatalf("second rule: %s", body)
	}

	// The list: ordered, with names.
	resp, body = h.do(http.MethodGet, rules(1), nil, nil)
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(body), `"match":"DOMAIN-SUFFIX,youtube.com"`) || !strings.Contains(string(body), `"exit_name":"B"`) {
		t.Fatalf("list: %d %s", resp.StatusCode, body)
	}

	// Edited: another exit and another match; the inbound may be cleared with 0.
	rule := rules(1) + "/" + id(created.ID)
	resp, body = h.do(http.MethodPatch, rule, map[string]any{"match": "IP-CIDR,203.0.113.0/24", "inbound_id": 0}, csrf)
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(body), `"match":"IP-CIDR,203.0.113.0/24"`) || strings.Contains(string(body), `"inbound_id"`) {
		t.Fatalf("patch: %d %s", resp.StatusCode, body)
	}
	if resp, rb := h.do(http.MethodPatch, rule, map[string]any{"match": "not a rule"}, csrf); resp.StatusCode != http.StatusUnprocessableEntity || !strings.Contains(string(rb), `"match"`) {
		t.Fatalf("patch bad match: %d %s", resp.StatusCode, rb)
	}
	if resp, _ := h.do(http.MethodPatch, rules(1)+"/"+id(9999), map[string]any{"match": "DST-PORT,25"}, csrf); resp.StatusCode != http.StatusNotFound {
		t.Fatalf("patch missing: %d", resp.StatusCode)
	}

	// Order: the whole list once, else refused; then the positions follow.
	if resp, _ := h.do(http.MethodPut, rules(1)+"/order", map[string]any{"ids": []int64{created.ID}}, csrf); resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("short order: %d", resp.StatusCode)
	}
	if resp, _ := h.do(http.MethodPut, rules(1)+"/order", map[string]any{"ids": []int64{second.ID, created.ID}}, csrf); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("order: %d", resp.StatusCode)
	}
	resp, body = h.do(http.MethodGet, rules(1), nil, nil)
	if !strings.Contains(string(body), `"position":1`) || strings.Index(string(body), `"match":"DST-PORT,25-53"`) > strings.Index(string(body), `"match":"IP-CIDR,203.0.113.0/24"`) {
		t.Fatalf("after order: %s", body)
	}

	// Deleted: gone from the list, then 404.
	if resp, _ := h.do(http.MethodDelete, rules(1)+"/"+id(second.ID), nil, csrf); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete: %d", resp.StatusCode)
	}
	if resp, _ := h.do(http.MethodDelete, rules(1)+"/"+id(second.ID), nil, csrf); resp.StatusCode != http.StatusNotFound {
		t.Fatalf("delete again: %d", resp.StatusCode)
	}
	resp, body = h.do(http.MethodGet, rules(1), nil, nil)
	if strings.Contains(string(body), "DST-PORT") || !strings.Contains(string(body), "IP-CIDR") {
		t.Fatalf("after delete: %s", body)
	}
}
