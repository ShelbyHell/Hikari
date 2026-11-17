package nodeapi

import "testing"

func TestCanonMatch(t *testing.T) {
	ok := map[string]string{
		"DOMAIN-SUFFIX,Example.COM":   "DOMAIN-SUFFIX,example.com",
		"domain-suffix, *.example.org": "DOMAIN-SUFFIX,example.org",
		"DOMAIN,пример.рф":             "DOMAIN,xn--e1afmkfd.xn--p1ai",
		"DOMAIN-KEYWORD, torrent":     "DOMAIN-KEYWORD,torrent",
		"DOMAIN-REGEX,^cdn\\d+\\.":    "DOMAIN-REGEX,^cdn\\d+\\.",
		"DST-PORT, 25":                "DST-PORT,25",
		"src-port,1000 - 2000":        "SRC-PORT,1000-2000",
		"IP-CIDR,203.0.113.7":         "IP-CIDR,203.0.113.7/32",
		"ip-cidr, 203.0.113.0/24 ":    "IP-CIDR,203.0.113.0/24",
		"IP-CIDR,2606:4700::/32":      "IP-CIDR6,2606:4700::/32",
		"SRC-IP-CIDR,10.0.0.0/8":      "IP-CIDR,10.0.0.0/8",
	}
	for in, want := range ok {
		got, ok := CanonMatch(in)
		if !ok || got != want {
			t.Errorf("CanonMatch(%q) = %q, %v; want %q, true", in, got, ok, want)
		}
	}
	bad := []string{
		"", "example.com", "DOMAIN-SUFFIX,", ",example.com",
		"GEOSITE,youtube", "GEOIP,cn", "RULE-SET,mikan-ai", // no geodata on the node
		"DOMAIN-SUFFIX,evil.com,DIRECT", "DOMAIN-SUFFIX,bad host",
		"DST-PORT,0", "DST-PORT,70000", "DST-PORT,25,53",
		"IP-CIDR,not-a-net", "IP-CIDR,example.com",
		"DOMAIN-SUFFIX," + string(make([]byte, 600)),
		"PROCESS-NAME,chrome", "IN-NAME,in-vless",
	}
	for _, in := range bad {
		if got, ok := CanonMatch(in); ok {
			t.Errorf("CanonMatch(%q) = %q, true; want refused", in, got)
		}
	}
}
