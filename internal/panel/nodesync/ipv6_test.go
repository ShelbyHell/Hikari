package nodesync

import (
	"context"
	"testing"

	"mikan/internal/panel/store/db"
)

// The IPv6 setting of a node reaches its DesiredState: on by default,
// and when turned off, stateKey changes and st.IPv6 is false.
func TestIPv6InDesiredState(t *testing.T) {
	s, _, st, _, _ := setup(t)
	ctx := context.Background()

	on, err := s.desired(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if on.IPv6 == nil || !*on.IPv6 {
		t.Fatalf("expected IPv6 on by default, got %v", on.IPv6)
	}

	// Turn off IPv6 on the node.
	if err := st.Q.SetNodeIPv6(ctx, db.SetNodeIPv6Params{Ipv6: 0, UpdatedAt: 1234, ID: s.id}); err != nil {
		t.Fatal(err)
	}
	s.m.SlotsChanged()

	off, err := s.desired(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if off.IPv6 == nil || *off.IPv6 {
		t.Fatalf("expected IPv6 off, got %v", off.IPv6)
	}

	if stateKey(on) == stateKey(off) {
		t.Fatal("toggling IPv6 did not change the state key")
	}
}
