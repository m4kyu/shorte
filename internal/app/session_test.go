package app

import (
	"net/http/httptest"
	"testing"
)

func TestSessionRoundTrip(t *testing.T) {
	rr := httptest.NewRecorder()
	setSession(rr, "secret-key-123456", 42)

	req := httptest.NewRequest("GET", "/", nil)
	for _, c := range rr.Result().Cookies() {
		req.AddCookie(c)
	}
	v, ok := getSession(req, "secret-key-123456")
	if !ok || v != "42" {
		t.Fatalf("unexpected session value: %q ok=%v", v, ok)
	}
}

