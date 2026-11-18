package node

import (
	"encoding/json"
	"fmt"
	"strings"

	"mikan/internal/nodeapi"
	"mikan/internal/proto"
)

// listenerNames are the listeners the state runs, the relay included: rules may name
// only these.
func listenerNames(st nodeapi.DesiredState) map[string]bool {
	present := map[string]bool{}
	for _, in := range st.Inbounds {
		present[in.Name] = true
	}
	if st.Relay != nil {
		present[nodeapi.RelayListener] = true
	}
	return present
}

// relayListener is the hidden VLESS REALITY door other nodes of the panel come in by.
func relayListener(r *nodeapi.Relay, cert proto.Cert) (map[string]any, error) {
	t, err := proto.FromJSON(r.Config)
	if err != nil {
		return nil, err
	}
	if t.Type() != "vless" || t["reality-config"] == nil {
		return nil, fmt.Errorf("the relay is VLESS REALITY")
	}
	return proto.Listener(t, nodeapi.RelayListener, "", r.Port, r.Users, cert, proto.Options{})
}

// exitProxyConfig is the outbound to another node's relay. The panel builds it; the node
// only names it and keeps it to the protocol relays speak.
func exitProxyConfig(e nodeapi.Exit) (map[string]any, error) {
	if !strings.HasPrefix(e.Name, "NODE-") || !safeRuleValue(e.Name) {
		return nil, fmt.Errorf("exit name %q", e.Name)
	}
	var p map[string]any
	if err := json.Unmarshal(e.Proxy, &p); err != nil {
		return nil, fmt.Errorf("exit %s: %w", e.Name, err)
	}
	if p["type"] != "vless" {
		return nil, fmt.Errorf("exit %s: the relay is VLESS", e.Name)
	}
	p["name"] = e.Name
	return p, nil
}

// cascadeRules send matching traffic to the exits' nodes, before the inbounds' catch-all
// exits apply: the first rule that matches wins. What does not parse, or names a listener,
// an exit, or an unconfigured rule set that is not there, is left out: mihomo refuses the whole
// config over one rule it cannot parse.
func cascadeRules(st nodeapi.DesiredState) []string {
	present := listenerNames(st)
	exits := map[string]bool{}
	for _, e := range st.Exits {
		if strings.HasPrefix(e.Name, "NODE-") && safeRuleValue(e.Name) {
			exits[e.Name] = true
		}
	}
	providers := map[string]bool{}
	for _, p := range st.RuleProviders {
		providers[p.Name] = true
	}
	var r []string
	for _, cr := range st.CascadeRules {
		match, ok := nodeapi.CanonMatch(cr.Match)
		if !ok || !exits[cr.Exit] || !safeRuleValue(cr.Exit) {
			continue
		}
		noResolve := false
		typ, val := nodeapi.SplitMatch(match)
		if typ == "RULE-SET" {
			name, opt, hasOpt := strings.Cut(val, ",")
			if hasOpt && strings.TrimSpace(opt) == "no-resolve" {
				noResolve = true
			}
			name = strings.TrimSpace(name)
			if !providers[name] {
				continue
			}
		}
		if cr.Inbound != "" {
			if !present[cr.Inbound] || !safeRuleValue(cr.Inbound) {
				continue
			}
			r = append(r, "AND,((IN-NAME,"+cr.Inbound+"),("+match+")),"+cr.Exit)
		} else if noResolve {
			name, _, _ := strings.Cut(val, ",")
			r = append(r, "RULE-SET,"+strings.TrimSpace(name)+","+cr.Exit+",no-resolve")
		} else {
			r = append(r, match+","+cr.Exit)
		}
	}
	return r
}

// exitRules send the inbounds of each exit to that node. An inbound named by several
// exits goes to the first.
func exitRules(st nodeapi.DesiredState) []string {
	present := listenerNames(st)
	taken := map[string]bool{}
	var r []string
	for _, e := range st.Exits {
		if !strings.HasPrefix(e.Name, "NODE-") || !safeRuleValue(e.Name) {
			continue
		}
		for _, name := range e.Inbounds {
			if present[name] && !taken[name] && safeRuleValue(name) {
				taken[name] = true
				r = append(r, "IN-NAME,"+name+","+e.Name)
			}
		}
	}
	return r
}
