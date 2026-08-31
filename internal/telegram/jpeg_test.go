package telegram

import (
	"os"
	"testing"
)

func TestToJPEGFromWebP(t *testing.T) {
	data, err := os.ReadFile("testdata/photo.webp")
	if err != nil {
		t.Fatal(err)
	}
	jpg, err := toJPEG(data)
	if err != nil {
		t.Fatalf("toJPEG: %v", err)
	}
	if len(jpg) == 0 {
		t.Fatal("empty jpeg")
	}
	// JPEG SOI marker
	if jpg[0] != 0xFF || jpg[1] != 0xD8 {
		t.Fatalf("not a jpeg: %x %x", jpg[0], jpg[1])
	}
}

func TestToJPEGInvalid(t *testing.T) {
	if _, err := toJPEG([]byte("not a webp")); err == nil {
		t.Fatal("expected error for invalid input")
	}
}
