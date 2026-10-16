// Package parsers reads the subscriptions of an upstream feed: what the panel takes when
// the admin attaches a feed to a tariff, and turns into servers of its own.
package parsers

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"

	"mikan/internal/proto"
)

// Server is one server of a feed: Name as the feed named it, Link the share link for
// Xray-family apps ("" when the feed gave a mihomo map only), Mihomo the proxy map for
// Clash-family apps; nil when the feed's format is not one the panel re-serves.
type Server struct {
	Name    string
	Link    string
	Enabled bool
	Mihomo  map[string]any
}

// ParseSubscription reads a feed body in the formats panels usually serve: a mihomo
// (Clash) profile, a base64 list of share links, or the share links as they are. The
// first format that yields servers wins; a feed in none of them is refused with the
// reason, so the admin sees a bad link at once instead of a subscription with no servers.
func ParseSubscription(body []byte, _ string) ([]Server, error) {
	for _, decode := range []func([]byte) ([]Server, bool){clashProxies, shareLinks} {
		if servers, ok := decode(body); ok && len(servers) > 0 {
			return servers, nil
		}
	}
	// The body may be base64 of either (panels base64 the link list most often).
	plain := base64Body(body)
	if len(plain) > 0 {
		for _, decode := range []func([]byte) ([]Server, bool){clashProxies, shareLinks} {
			if servers, ok := decode(plain); ok && len(servers) > 0 {
				return servers, nil
			}
		}
	}
	return nil, fmt.Errorf("the feed holds no servers the panel can re-serve (a mihomo profile or share links)")
}

// base64Body decodes a body that is base64 with the line breaks feeds put in; "" when it
// is not base64 at all.
func base64Body(body []byte) []byte {
	src := strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == ' ' {
			return -1
		}
		return r
	}, string(body))
	if src == "" || len(src)%4 != 0 {
		return nil
	}
	decoded := make([]byte, base64.StdEncoding.DecodedLen(len(src)))
	n, err := base64.StdEncoding.Decode(decoded, []byte(src))
	if err != nil || n == 0 {
		return nil
	}
	return decoded[:n]
}

// clashProxies reads the "proxies:" of a mihomo profile: the maps are already what
// Clash-family apps take, they go through with their names checked.
func clashProxies(body []byte) ([]Server, bool) {
	var cfg struct {
		Proxies []map[string]any `yaml:"proxies"`
	}
	if err := yaml.Unmarshal(body, &cfg); err != nil || len(cfg.Proxies) == 0 {
		return nil, false
	}
	var out []Server
	for _, m := range cfg.Proxies {
		name, _ := m["name"].(string)
		typ, _ := m["type"].(string)
		if name == "" || typ == "" || m["server"] == nil || m["port"] == nil {
			continue // a proxy without the minimum is not a server; the rest still go
		}
		out = append(out, Server{Name: name, Enabled: true, Mihomo: realityFixup(m)})
	}
	return out, len(out) > 0
}

// realityFixup makes a feed's REALITY proxy dialable by mihomo clients: a proxy with
// reality-opts connects only through uTLS, so one without a fingerprint mihomo knows
// gets the panel's default, and a short-id YAML read as a number (they are all digits
// as often as not) goes back to text, which older client cores require.
func realityFixup(m map[string]any) map[string]any {
	reality, ok := m["reality-opts"].(map[string]any)
	if !ok || len(reality) == 0 {
		return m
	}
	switch sid := reality["short-id"].(type) {
	case string:
	case int:
		reality["short-id"] = strconv.Itoa(sid)
	case int64:
		reality["short-id"] = strconv.FormatInt(sid, 10)
	case uint64:
		reality["short-id"] = strconv.FormatUint(sid, 10)
	case float64:
		reality["short-id"] = strconv.FormatFloat(sid, 'f', -1, 64)
	}
	if fp, _ := m["client-fingerprint"].(string); !proto.KnownFingerprint(fp) {
		m["client-fingerprint"] = proto.DefaultFingerprint
	}
	return m
}

// shareLinks reads the links of the protocols the panel re-serves. A link that parses
// into no mihomo map is left out: a client must never receive a proxy it cannot run.
func shareLinks(body []byte) ([]Server, bool) {
	var out []Server
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		s, ok := shareLink(line)
		if !ok {
			continue
		}
		out = append(out, s)
	}
	return out, len(out) > 0
}

// shareLink parses one share link; ok is false for the kinds the panel does not re-serve.
func shareLink(link string) (Server, bool) {
	switch {
	case strings.HasPrefix(link, "vmess://"):
		return vmessLink(link)
	case strings.HasPrefix(link, "vless://"):
		return tlsLink(link, "vless")
	case strings.HasPrefix(link, "trojan://"):
		return tlsLink(link, "trojan")
	case strings.HasPrefix(link, "ss://"):
		return ssLink(link)
	case strings.HasPrefix(link, "hysteria2://"), strings.HasPrefix(link, "hy2://"):
		return hysteria2Link(link)
	case strings.HasPrefix(link, "tuic://"):
		return tuicLink(link)
	}
	return Server{}, false
}

// nameOf takes the fragment a link names its server with.
func nameOf(u *url.URL, fallback string) string {
	if name := strings.TrimSpace(u.Fragment); name != "" {
		return name
	}
	return fallback
}

// vmessLink reads vmess://base64(json), the format V2RayN made common.
func vmessLink(link string) (Server, bool) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(link, "vmess://"))
	if err != nil {
		return Server{}, false
	}
	var v struct {
		Name   string          `json:"ps"`
		Server string          `json:"add"`
		Port   json.RawMessage `json:"port"`
		UUID   string          `json:"id"`
		AID    json.RawMessage `json:"aid"`
		Cipher string          `json:"scy"`
		Net    string          `json:"net"`
		Host   string          `json:"host"`
		Path   string          `json:"path"`
		TLS    string          `json:"tls"`
		SNI    string          `json:"sni"`
	}
	if err := json.Unmarshal(raw, &v); err != nil || v.Server == "" || v.UUID == "" {
		return Server{}, false
	}
	port, _ := strconv.Atoi(strings.Trim(string(v.Port), `"`))
	if port <= 0 || port > 65535 {
		return Server{}, false
	}
	aid := 0
	_ = json.Unmarshal(v.AID, &aid)
	cipher := v.Cipher
	if cipher == "" {
		cipher = "auto"
	}
	m := map[string]any{"name": v.Name, "type": "vmess", "server": v.Server, "port": port,
		"uuid": v.UUID, "alterId": aid, "cipher": cipher, "udp": true}
	applyTransport(m, v.Net, v.Host, v.Path, v.TLS == "tls" || v.TLS == "reality", v.SNI)
	return Server{Name: v.Name, Link: link, Enabled: true, Mihomo: m}, true
}

// tlsLink reads vless:// and trojan://: userinfo@host:port with query options.
func tlsLink(link, typ string) (Server, bool) {
	u, err := url.Parse(link)
	if err != nil || u.Host == "" {
		return Server{}, false
	}
	host, portS := splitHostPort(u.Host)
	port, _ := strconv.Atoi(portS)
	if port <= 0 || port > 65535 {
		return Server{}, false
	}
	q := u.Query()
	name := nameOf(u, host+":"+portS)
	m := map[string]any{"name": name, "type": typ, "server": host, "port": port, "udp": true}
	if typ == "vless" {
		if u.User == nil || u.User.Username() == "" {
			return Server{}, false
		}
		m["uuid"] = u.User.Username()
		if flow := q.Get("flow"); flow != "" {
			m["flow"] = flow
		}
	} else {
		if pw, ok := u.User.Password(); ok && pw != "" {
			m["password"] = pw
		} else if u.User.Username() != "" {
			m["password"] = u.User.Username()
		} else {
			return Server{}, false
		}
	}
	security := q.Get("security")
	if security == "reality" {
		m["tls"] = true
		reality := map[string]any{}
		if pbk := q.Get("pbk"); pbk != "" {
			reality["public-key"] = pbk
		}
		if sid := q.Get("sid"); sid != "" {
			reality["short-id"] = sid
		}
		if len(reality) > 0 {
			m["reality-opts"] = reality
			// A mihomo client dials REALITY through uTLS and breaks the connection without
			// a fingerprint it knows; Xray apps fall back to chrome on their own, so the
			// feed's links without fp (or with fp=none) work there and only here. The
			// link's own fingerprint stays when mihomo knows it.
			if fp := q.Get("fp"); proto.KnownFingerprint(fp) {
				m["client-fingerprint"] = fp
			} else {
				m["client-fingerprint"] = proto.DefaultFingerprint
			}
		}
	} else if security == "tls" || q.Get("sni") != "" || port == 443 {
		m["tls"] = true
	}
	applyQueryTLS(m, q)
	applyTransport(m, q.Get("type"), q.Get("host"), q.Get("path"), m["tls"] == true, q.Get("sni"))
	return Server{Name: name, Link: link, Enabled: true, Mihomo: m}, true
}

// ssLink reads ss://, both the SIP002 form (userinfo@host:port) and the legacy
// ss://base64(method:password@host:port).
func ssLink(link string) (Server, bool) {
	u, err := url.Parse(link)
	if err != nil || u.Host == "" {
		return Server{}, false
	}
	var method, password string
	if u.User != nil {
		method = u.User.Username()
		password, _ = u.User.Password()
		if method == "" {
			return Server{}, false
		}
		// SIP002 keeps the method plain and the password possibly plain too.
		if password == "" {
			method, password = decodeUserInfo(method)
		}
	} else {
		raw, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(link, "ss://"))
		if err != nil {
			raw, err = base64.StdEncoding.DecodeString(strings.TrimPrefix(link, "ss://"))
		}
		if err != nil {
			return Server{}, false
		}
		head, _, ok := strings.Cut(string(raw), "@")
		if !ok {
			return Server{}, false
		}
		method, password, _ = strings.Cut(head, ":")
	}
	host, portS := splitHostPort(u.Host)
	port, _ := strconv.Atoi(portS)
	if port <= 0 || port > 65535 || method == "" || password == "" {
		return Server{}, false
	}
	name := nameOf(u, host+":"+portS)
	m := map[string]any{"name": name, "type": "ss", "server": host, "port": port,
		"cipher": method, "password": password, "udp": true}
	plugin := u.Query().Get("plugin")
	if p := parsePlugin(plugin); p != nil {
		m["plugin"] = p["plugin"]
		m["plugin-opts"] = p["opts"]
	}
	return Server{Name: name, Link: link, Enabled: true, Mihomo: m}, true
}

// hysteria2Link reads hysteria2://password@host:port?sni=&insecure=.
func hysteria2Link(link string) (Server, bool) {
	u, err := url.Parse(link)
	if err != nil || u.Host == "" {
		return Server{}, false
	}
	host, portS := splitHostPort(u.Host)
	port, _ := strconv.Atoi(portS)
	if port <= 0 || port > 65535 {
		return Server{}, false
	}
	auth := ""
	if u.User != nil {
		auth = u.User.Username()
	}
	if auth == "" {
		return Server{}, false
	}
	q := u.Query()
	name := nameOf(u, host+":"+portS)
	m := map[string]any{"name": name, "type": "hysteria2", "server": host, "port": port, "password": auth}
	applyQueryTLS(m, q)
	return Server{Name: name, Link: link, Enabled: true, Mihomo: m}, true
}

// tuicLink reads tuic://uuid:password@host:port?congestion_control=&alpn=.
func tuicLink(link string) (Server, bool) {
	u, err := url.Parse(link)
	if err != nil || u.Host == "" || u.User == nil {
		return Server{}, false
	}
	host, portS := splitHostPort(u.Host)
	port, _ := strconv.Atoi(portS)
	if port <= 0 || port > 65535 {
		return Server{}, false
	}
	pw, _ := u.User.Password()
	if u.User.Username() == "" {
		return Server{}, false
	}
	q := u.Query()
	name := nameOf(u, host+":"+portS)
	m := map[string]any{"name": name, "type": "tuic", "server": host, "port": port,
		"uuid": u.User.Username(), "password": pw}
	if cc := q.Get("congestion_control"); cc != "" {
		m["congestion-controller"] = cc
	}
	if alpn := q.Get("alpn"); alpn != "" {
		m["alpn"] = strings.Split(alpn, ",")
	}
	applyQueryTLS(m, q)
	return Server{Name: name, Link: link, Enabled: true, Mihomo: m}, true
}

// applyQueryTLS carries the sni/skip-cert-verify/fingerprint query options every
// TLS-speaking protocol shares into the map. A fingerprint a REALITY proxy already got
// stays: the link's own fp may be one mihomo clients cannot dial REALITY with.
func applyQueryTLS(m map[string]any, q url.Values) {
	if sni := q.Get("sni"); sni != "" {
		m["sni"] = sni
	}
	if q.Get("insecure") == "1" || q.Get("allowInsecure") == "1" {
		m["skip-cert-verify"] = true
	}
	if fp := q.Get("fp"); fp != "" && m["client-fingerprint"] == nil {
		m["client-fingerprint"] = fp
	}
}

// applyTransport fills the network-specific options (ws/grpc/tcp) a link or a vmess
// record names; a network the map cannot express keeps the plain TCP defaults.
func applyTransport(m map[string]any, net, host, path string, tls bool, sni string) {
	if sni != "" && tls {
		m["sni"] = sni
	}
	switch net {
	case "ws":
		m["network"] = "ws"
		opts := map[string]any{}
		if path != "" {
			opts["path"] = path
		}
		if host != "" {
			opts["headers"] = map[string]any{"Host": host}
		}
		if len(opts) > 0 {
			m["ws-opts"] = opts
		}
	case "grpc":
		m["network"] = "grpc"
		if host != "" { // the service name rides in host for grpc links
			m["grpc-opts"] = map[string]any{"grpc-service-name": host}
		}
	}
}

// parsePlugin reads a SIP003 plugin parameter ("obfs-local;obfs=http;obfs-host=x") into
// the plugin and its options; nil when the panel re-serves no such plugin.
func parsePlugin(plugin string) map[string]any {
	if plugin == "" {
		return nil
	}
	parts := strings.Split(plugin, ";")
	switch parts[0] {
	case "obfs-local", "simple-obfs":
		p := map[string]any{"plugin": "obfs", "opts": map[string]any{}}
		opts := p["opts"].(map[string]any)
		for i := 1; i+1 < len(parts); i += 2 {
			switch parts[i] {
			case "obfs":
				opts["mode"] = parts[i+1]
			case "obfs-host":
				opts["host"] = parts[i+1]
			}
		}
		return p
	case "v2ray-plugin":
		p := map[string]any{"plugin": "v2ray-plugin", "opts": map[string]any{}}
		opts := p["opts"].(map[string]any)
		for _, part := range parts[1:] {
			switch part {
			case "tls":
				opts["tls"] = true
			case "host":
			default:
				if k, v, ok := strings.Cut(part, "="); ok && k == "host" {
					opts["host"] = v
				}
			}
		}
		return p
	}
	return nil
}

// decodeUserInfo decodes a base64 userinfo of a legacy ss link.
func decodeUserInfo(s string) (method, password string) {
	for _, enc := range []*base64.Encoding{base64.RawURLEncoding, base64.RawStdEncoding, base64.StdEncoding} {
		if raw, err := enc.DecodeString(s); err == nil {
			m, p, _ := strings.Cut(string(raw), ":")
			return m, p
		}
	}
	return "", ""
}

// splitHostPort splits "host:port", taking the bracketed IPv6 form apart too.
func splitHostPort(hostport string) (host, port string) {
	if strings.HasPrefix(hostport, "[") {
		if end := strings.Index(hostport, "]"); end != -1 {
			host = hostport[1:end]
			if rest := hostport[end+1:]; strings.HasPrefix(rest, ":") {
				port = rest[1:]
			}
			return host, port
		}
	}
	host, port, _ = strings.Cut(hostport, ":")
	return host, port
}
