package clankerwatch

import "testing"

func TestNormalizeSessionSlug(t *testing.T) {
	got, err := NormalizeSessionSlug("StoreFront-Orders")
	if err != nil {
		t.Fatal(err)
	}
	if got != "storefront-orders" {
		t.Fatalf("slug = %q, want storefront-orders", got)
	}

	if _, err := NormalizeSessionSlug(""); err == nil {
		t.Fatal("expected empty slug error")
	}
	if _, err := NormalizeSessionSlug("has space"); err == nil {
		t.Fatal("expected whitespace error")
	}
	if _, err := NormalizeSessionSlug("bad_slug"); err == nil {
		t.Fatal("expected underscore error")
	}
	if _, err := NormalizeSessionSlug(stringsRepeat("a", 101)); err == nil {
		t.Fatal("expected length error")
	}
}

func stringsRepeat(s string, n int) string {
	out := ""
	for i := 0; i < n; i++ {
		out += s
	}
	return out
}
