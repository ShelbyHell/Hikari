package parsers

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestClashProxies(t *testing.T) {
	body := []byte("port: 7890\nproxies:\n  - {name: 🇳🇱 NL-1, type: vless, server: 10.0.0.1, port: 443, uuid: 11111111-1111-1111-1111-111111111111, tls: true}\n  - name: NL-2\n    type: trojan\n    server: 10.0.0.2\n    port: 443\n    password: pw\n")
	servers, err := ParseSubscription(body, "https://example.com/sub")
	if err != nil {
		t.Fatal(err)
	}
	if len(servers) != 2 {
		t.Fatalf("want 2 servers, got %d", len(servers))
	}
	if servers[0].Name != "🇳🇱 NL-1" || servers[0].Mihomo["type"] != "vless" {
		t.Fatalf("bad first server: %v", servers[0])
	}
	if servers[0].Link != "" {
		t.Fatalf("a Clash feed gives no share links, got %q", servers[0].Link)
	}
}

func TestBase64ShareLinks(t *testing.T) {
	links := strings.Join([]string{
		"trojan://pw@10.0.0.3:443?sni=example.com#NL-3",
		"hysteria2://auth@10.0.0.4:443?sni=example.com&insecure=1#NL-4",
		"tuic://11111111-1111-1111-1111-111111111111:pw@10.0.0.5:443?congestion_control=bbr&alpn=h3#NL-5",
		"ss://" + base64.RawURLEncoding.EncodeToString([]byte("aes-256-gcm:secret")) + "@10.0.0.6:8388#NL-6",
		"vless://11111111-1111-1111-1111-111111111111@10.0.0.7:443?security=reality&pbk=PUBKEY&sid=ab&fp=chrome&flow=xtls-rprx-vision#NL-7",
	}, "\n")
	b64 := base64.StdEncoding.EncodeToString([]byte(links))
	servers, err := ParseSubscription([]byte(b64), "https://example.com/sub")
	if err != nil {
		t.Fatal(err)
	}
	if len(servers) != 5 {
		t.Fatalf("want 5 servers, got %d: %v", len(servers), names(servers))
	}
	byName := map[string]Server{}
	for _, s := range servers {
		byName[s.Name] = s
	}
	trojan := byName["NL-3"]
	if trojan.Mihomo["type"] != "trojan" || trojan.Mihomo["password"] != "pw" || trojan.Mihomo["sni"] != "example.com" {
		t.Fatalf("trojan: %v", trojan.Mihomo)
	}
	if trojan.Link == "" {
		t.Fatal("a share link keeps its link for Xray apps")
	}
	hy2 := byName["NL-4"]
	if hy2.Mihomo["type"] != "hysteria2" || hy2.Mihomo["skip-cert-verify"] != true {
		t.Fatalf("hysteria2: %v", hy2.Mihomo)
	}
	tuic := byName["NL-5"]
	if tuic.Mihomo["type"] != "tuic" || tuic.Mihomo["congestion-controller"] != "bbr" {
		t.Fatalf("tuic: %v", tuic.Mihomo)
	}
	ss := byName["NL-6"]
	if ss.Mihomo["cipher"] != "aes-256-gcm" || ss.Mihomo["password"] != "secret" {
		t.Fatalf("ss: %v", ss.Mihomo)
	}
	vless := byName["NL-7"]
	ro, ok := vless.Mihomo["reality-opts"].(map[string]any)
	if vless.Mihomo["type"] != "vless" || !ok || ro["public-key"] != "PUBKEY" || ro["short-id"] != "ab" {
		t.Fatalf("vless: %v", vless.Mihomo)
	}
	if vless.Mihomo["flow"] != "xtls-rprx-vision" {
		t.Fatalf("vless flow: %v", vless.Mihomo)
	}
}

func TestVmessLink(t *testing.T) {
	raw := `{"ps":"NL-8","add":"10.0.0.8","port":"443","id":"11111111-1111-1111-1111-111111111111","aid":"0","scy":"auto","net":"ws","host":"cdn.example.com","path":"/ws","tls":"tls","sni":"cdn.example.com"}`
	link := "vmess://" + base64.StdEncoding.EncodeToString([]byte(raw))
	servers, err := ParseSubscription([]byte(link), "https://example.com/sub")
	if err != nil {
		t.Fatal(err)
	}
	if len(servers) != 1 {
		t.Fatalf("want 1 server, got %d", len(servers))
	}
	m := servers[0].Mihomo
	if m["type"] != "vmess" || m["cipher"] != "auto" || m["network"] != "ws" {
		t.Fatalf("vmess: %v", m)
	}
	ws, ok := m["ws-opts"].(map[string]any)
	if !ok || ws["path"] != "/ws" {
		t.Fatalf("vmess ws-opts: %v", m)
	}
}

func TestUnparseableIsRefused(t *testing.T) {
	if _, err := ParseSubscription([]byte("<html>hello</html>"), "https://example.com/sub"); err == nil {
		t.Fatal("an HTML page is not a subscription")
	}
}

func TestBrokenLinksAreLeftOut(t *testing.T) {
	body := "trojan://pw@10.0.0.9:443#OK\ntrojan://10.0.0.10:443#NO-PASSWORD\n"
	servers, err := ParseSubscription([]byte(body), "https://example.com/sub")
	if err != nil {
		t.Fatal(err)
	}
	if len(servers) != 1 || servers[0].Name != "OK" {
		t.Fatalf("want only the good link, got %v", names(servers))
	}
}

func names(servers []Server) []string {
	var out []string
	for _, s := range servers {
		out = append(out, s.Name)
	}
	return out
}
