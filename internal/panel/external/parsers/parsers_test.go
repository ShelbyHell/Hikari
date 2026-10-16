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
	if vless.Mihomo["client-fingerprint"] != "chrome" {
		t.Fatalf("vless keeps the link's fingerprint: %v", vless.Mihomo["client-fingerprint"])
	}
}

func TestRealityFingerprint(t *testing.T) {
	const pbk = "kBPI_JRK3J0k3ShNb_g_6R1H4G4sdvIdC6V4hxGjyAw"
	links := []string{
		// No fp, fp=none, and an own profile of another panel: a mihomo client dials
		// REALITY through uTLS and breaks the connection with each of these.
		"vless://11111111-1111-1111-1111-111111111111@10.0.0.20:443?security=reality&pbk=" + pbk + "&sid=ab#NO-FP",
		"vless://11111111-1111-1111-1111-111111111111@10.0.0.20:443?security=reality&pbk=" + pbk + "&sid=ab&fp=none#FP-NONE",
		"vless://11111111-1111-1111-1111-111111111111@10.0.0.20:443?security=reality&pbk=" + pbk + "&sid=ab&fp=myownprofile#FP-OWN",
		"trojan://pw@10.0.0.21:443?security=reality&pbk=" + pbk + "&sid=ab#TROJAN-REALITY",
	}
	for _, link := range links {
		servers, err := ParseSubscription([]byte(link), "https://example.com/sub")
		if err != nil || len(servers) != 1 {
			t.Fatalf("%s: %v, %v", link, servers, err)
		}
		if servers[0].Mihomo["client-fingerprint"] != "chrome" {
			t.Fatalf("%s: client-fingerprint %v, want chrome", link, servers[0].Mihomo["client-fingerprint"])
		}
	}
	// A fingerprint mihomo knows rides along.
	link := "vless://11111111-1111-1111-1111-111111111111@10.0.0.20:443?security=reality&pbk=" + pbk + "&sid=ab&fp=firefox#FP-KNOWN"
	servers, _ := ParseSubscription([]byte(link), "https://example.com/sub")
	if servers[0].Mihomo["client-fingerprint"] != "firefox" {
		t.Fatalf("firefox lost: %v", servers[0].Mihomo["client-fingerprint"])
	}
	// Plain TLS keeps the old shape: fp only when the link names it.
	links = []string{
		"vless://11111111-1111-1111-1111-111111111111@10.0.0.22:443?security=tls#PLAIN",
		"trojan://pw@10.0.0.22:443?sni=example.com&fp=safari#PLAIN-FP",
	}
	for _, link := range links {
		servers, err := ParseSubscription([]byte(link), "https://example.com/sub")
		if err != nil || len(servers) != 1 {
			t.Fatalf("%s: %v, %v", link, servers, err)
		}
		fp, has := servers[0].Mihomo["client-fingerprint"]
		if link[strings.Index(link, "#")+1:] == "PLAIN" && has {
			t.Fatalf("plain TLS grew a fingerprint: %v", servers[0].Mihomo)
		}
		if strings.HasSuffix(link, "PLAIN-FP") && fp != "safari" {
			t.Fatalf("plain TLS fp: %v", fp)
		}
	}
}

func TestClashRealityFixup(t *testing.T) {
	body := []byte(`proxies:
  - {name: R1, type: vless, server: 10.0.0.30, port: 443, uuid: 11111111-1111-1111-1111-111111111111,
     tls: true, client-fingerprint: firefox, flow: xtls-rprx-vision,
     reality-opts: {public-key: kBPI_JRK3J0k3ShNb_g_6R1H4G4sdvIdC6V4hxGjyAw, short-id: 12345678}}
  - {name: R2, type: trojan, server: 10.0.0.31, port: 443, password: pw, tls: true,
     reality-opts: {public-key: kBPI_JRK3J0k3ShNb_g_6R1H4G4sdvIdC6V4hxGjyAw}}
`)
	servers, err := ParseSubscription(body, "https://example.com/sub")
	if err != nil {
		t.Fatal(err)
	}
	if len(servers) != 2 {
		t.Fatalf("want 2 servers, got %d", len(servers))
	}
	r1 := servers[0].Mihomo
	if r1["client-fingerprint"] != "firefox" {
		t.Fatalf("R1 keeps its fingerprint: %v", r1["client-fingerprint"])
	}
	if sid, ok := r1["reality-opts"].(map[string]any)["short-id"]; !ok || sid != "12345678" {
		t.Fatalf("R1 short-id %v of %T, want the text 12345678", sid, sid)
	}
	r2 := servers[1].Mihomo
	if r2["client-fingerprint"] != "chrome" {
		t.Fatalf("R2 grew no fingerprint: %v", r2["client-fingerprint"])
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
