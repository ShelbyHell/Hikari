package subs

import (
	"testing"
)

func TestShopRequestDeviceValidation(t *testing.T) {
	cases := []struct {
		name string
		req  shopRequest
		want bool
	}{
		{"valid device request", shopRequest{Device: true, Token: "test-token"}, true},
		{"device without token", shopRequest{Device: true, Token: ""}, false},
		{"device with promo", shopRequest{Device: true, Token: "test-token", PromoCode: "PROMO"}, false},
		{"valid tariff request", shopRequest{TariffID: 1, Token: "test-token"}, true},
		{"valid trial request", shopRequest{Trial: true}, true},
		{"trial with token", shopRequest{Trial: true, Token: "test-token"}, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.req.valid(); got != tc.want {
				t.Errorf("req.valid() = %v, want %v", got, tc.want)
			}
		})
	}
}
