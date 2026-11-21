package api

import (
	"slices"
	"testing"
)

func TestUserOnlineSorting(t *testing.T) {
	d1 := int64(10)
	d2 := int64(20)
	list := []UserOnlineView{
		{NodeID: 2, DeviceID: &d1},
		{NodeID: 1, DeviceID: &d2},
		{NodeID: 1, DeviceID: nil},
		{NodeID: 2, DeviceID: nil},
	}
	slices.SortFunc(list, func(a, b UserOnlineView) int {
		if a.NodeID != b.NodeID {
			return int(a.NodeID - b.NodeID)
		}
		if (a.DeviceID == nil) != (b.DeviceID == nil) {
			if a.DeviceID == nil {
				return -1
			}
			return 1
		}
		if a.DeviceID != nil && b.DeviceID != nil {
			return int(*a.DeviceID - *b.DeviceID)
		}
		return 0
	})
	if list[0].NodeID != 1 || list[0].DeviceID != nil {
		t.Fatalf("first item should be Node 1, own keys: %+v", list[0])
	}
	if list[1].NodeID != 1 || *list[1].DeviceID != d2 {
		t.Fatalf("second item should be Node 1, device %d: %+v", d2, list[1])
	}
	if list[2].NodeID != 2 || list[2].DeviceID != nil {
		t.Fatalf("third item should be Node 2, own keys: %+v", list[2])
	}
	if list[3].NodeID != 2 || *list[3].DeviceID != d1 {
		t.Fatalf("fourth item should be Node 2, device %d: %+v", d1, list[3])
	}
}
