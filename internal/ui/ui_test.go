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
