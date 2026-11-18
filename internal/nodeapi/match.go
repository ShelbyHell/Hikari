package nodeapi

import (
	"regexp"
	"strings"
)

// CanonMatch takes a cascade rule's "TYPE,VALUE" (no target) and returns its canonical
// form, or refuses it. The kinds are the ones a node's rules can carry without geodata:
// domains, ports, networks and domain regexes. The node must never write a rule mihomo
// cannot parse: one bad rule fails the whole config, so the panel keeps the canonical
// form and the node builds from the canonical form only.

var matchRegex = regexp.MustCompile(`^[A-Za-z0-9_.\-*+?()\[\]{}^$|\\]+$`)

// CanonMatch is the canonical form of one rule's match: "DOMAIN-SUFFIX,example.com",
// "DST-PORT,25-53", "IP-CIDR,203.0.113.0/24", "RULE-SET,mikan-ai". GEOSITE and GEOIP
// are left out: the node runs no geodata, and a rule the core cannot resolve fails the whole config there.
var ruleSetNameRegex = regexp.MustCompile(`^[A-Za-z0-9_.\-]+$`)

func validRuleSetName(name string) bool {
	if strings.HasPrefix(name, "https://") || strings.HasPrefix(name, "http://") {
		return len(name) <= 512 && !strings.ContainsAny(name, " \t\r\n\"'<>")
	}
	return len(name) <= 128 && ruleSetNameRegex.MatchString(name)
}

func CanonMatch(s string) (string, bool) {
	typ, val, ok := strings.Cut(strings.TrimSpace(s), ",")
	if !ok {
		return "", false
	}
	typ = strings.ToUpper(strings.TrimSpace(typ))
	val = strings.TrimSpace(val)
	if val == "" || len(val) > 512 || strings.ContainsAny(val, "\t\r\n") {
		return "", false
	}
	if typ != "RULE-SET" && strings.ContainsAny(val, ",") {
		return "", false
	}
	switch typ {
	case "DOMAIN", "DOMAIN-SUFFIX", "DOMAIN-KEYWORD":
		d, ok := CanonDomain(val)
		if !ok {
			return "", false
		}
		return typ + "," + d, true
	case "DOMAIN-REGEX":
		// A rule splits on commas; a regex with a space cannot be written one.
		if strings.ContainsAny(val, " ") || !matchRegex.MatchString(val) {
			return "", false
		}
		return typ + "," + val, true
	case "DST-PORT", "SRC-PORT":
		p, ok := CanonPorts(val)
		if !ok {
			return "", false
		}
		return typ + "," + p, true
	case "IP-CIDR", "SRC-IP-CIDR", "IP-CIDR6":
		p, ok := CanonNetwork(val)
		if !ok {
			return "", false
		}
		kind := "IP-CIDR"
		if p.Addr().Is6() {
			kind = "IP-CIDR6"
		}
		return kind + "," + p.String(), true
	case "RULE-SET":
		name, opt, hasOpt := strings.Cut(val, ",")
		name = strings.TrimSpace(name)
		if name == "" {
			return "", false
		}
		noResolve := false
		if hasOpt {
			if strings.ToLower(strings.TrimSpace(opt)) != "no-resolve" {
				return "", false
			}
			noResolve = true
		}
		if !validRuleSetName(name) {
			return "", false
		}
		if noResolve {
			return "RULE-SET," + name + ",no-resolve", true
		}
		return "RULE-SET," + name, true
	}
	return "", false
}

// MatchKinds are the rule types the panel offers for a cascade rule, in the UI's order.
var MatchKinds = []string{"DOMAIN-SUFFIX", "DOMAIN", "DOMAIN-KEYWORD", "DOMAIN-REGEX", "DST-PORT", "IP-CIDR", "RULE-SET"}

// SplitMatch is CanonMatch's input taken apart for the UI: the type and the value.
func SplitMatch(s string) (typ, val string) {
	typ, val, _ = strings.Cut(strings.TrimSpace(s), ",")
	return strings.ToUpper(strings.TrimSpace(typ)), strings.TrimSpace(val)
}
