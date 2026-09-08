package ui

import "testing"

func TestEnabled(t *testing.T) {
	if !Enabled("", true) {
		t.Fatal("expected color on TTY")
	}
	if Enabled("1", true) {
		t.Fatal("NO_COLOR disables")
	}
	if Enabled("", false) {
		t.Fatal("non-TTY disables")
	}
}

func TestBytes(t *testing.T) {
	if got := Bytes(100); got != "100 B" {
		t.Fatalf("got %q", got)
	}
	if got := Bytes(1024); got != "1.0 KB" {
		t.Fatalf("got %q", got)
	}
	if got := Bytes(1572864); got != "1.5 MB" {
		t.Fatalf("got %q", got)
	}
}
