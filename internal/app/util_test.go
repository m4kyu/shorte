package app

import "testing"

func TestValidateURL(t *testing.T) {
	cases := []struct {
		in   string
		pass bool
	}{
		{"https://example.com", true},
		{"http://example.com/path", true},
		{"ftp://example.com", false},
		{"javascript:alert(1)", false},
		{"", false},
	}
	for _, c := range cases {
		err := validateURL(c.in)
		if c.pass && err != nil {
			t.Fatalf("expected pass for %q, got %v", c.in, err)
		}
		if !c.pass && err == nil {
			t.Fatalf("expected fail for %q", c.in)
		}
	}
}

