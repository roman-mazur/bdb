package usb

import (
	"testing"
)

func TestBuildUsbDescriptors(t *testing.T) {
	res := BuildUsbDescriptors()
	t.Log(res)
	if le.Uint32(res) != functionfsDescriptorsMagicV2 {
		t.Error("incorrect magic")
	}
	if int(le.Uint32(res[4:])) != len(res) {
		t.Error("incorrect length in the descriptor")
	}
}

func TestBuildUsbStrings(t *testing.T) {
	res := BuildUsbStrings()
	t.Log(res)
	if le.Uint32(res) != functionfsStringsMagic {
		t.Error("incorrect magic")
	}
	if int(le.Uint32(res[4:])) != len(res) {
		t.Error("incorrect length in the descriptor")
	}
	if res[len(res)-1] != 0 {
		t.Error("non-null terminated string")
	}
}
