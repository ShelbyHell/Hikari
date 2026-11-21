package node

import (
	"bytes"
	"encoding/binary"
	"net/netip"
	"testing"
)

func TestProxyV2Header(t *testing.T) {
	src := netip.MustParseAddrPort("203.0.113.7:51234")
	dst := netip.MustParseAddrPort("198.51.100.1:443")
	b := proxyV2(src, dst)
	if !bytes.HasPrefix(b, []byte("\r\n\r\n\x00\r\nQUIT\n")) || b[12] != 0x21 || b[13] != 0x11 {
		t.Fatalf("invalid PROXY v2 header: %x", b)
	}
	if binary.BigEndian.Uint16(b[24:26]) != src.Port() || binary.BigEndian.Uint16(b[26:28]) != dst.Port() {
		t.Fatalf("invalid ports in PROXY v2 header")
	}
}

func TestJoinHello(t *testing.T) {
	plain := []byte{0x16, 0x03, 0x01, 0x00, 0x05, 0x01, 0x00, 0x00, 0x01, 0x00}
	msg, st := joinHello(plain)
	if st != helloDone || len(msg) != 5 {
		t.Fatalf("expected helloDone with 5-byte message, got %d and %d bytes", st, len(msg))
	}
}
