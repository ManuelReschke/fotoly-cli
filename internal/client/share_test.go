package client

import "testing"

func TestResolveShareURLRelative(t *testing.T) {
	got, err := ResolveShareURL("https://fotoly.eu", "/i/abc")
	if err != nil {
		t.Fatal(err)
	}
	if got != "https://fotoly.eu/i/abc" {
		t.Fatalf("got %q", got)
	}
}

func TestResolveShareURLAbsolute(t *testing.T) {
	got, err := ResolveShareURL("https://fotoly.eu", "https://cdn.example/i/abc")
	if err != nil {
		t.Fatal(err)
	}
	if got != "https://cdn.example/i/abc" {
		t.Fatalf("got %q", got)
	}
}

func TestResolveShareURLFirstNonEmpty(t *testing.T) {
	got, err := ResolveShareURL("https://fotoly.eu", "", "/i/from-view")
	if err != nil {
		t.Fatal(err)
	}
	if got != "https://fotoly.eu/i/from-view" {
		t.Fatalf("got %q", got)
	}
}

func TestResolveShareURLNone(t *testing.T) {
	_, err := ResolveShareURL("https://fotoly.eu")
	if err == nil {
		t.Fatal("expected error")
	}
}
