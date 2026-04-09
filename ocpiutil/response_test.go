package ocpiutil

import (
	"net/http/httptest"
	"testing"
)

func TestBuildLinkHeaderPrefersXRallyForwardedHost(t *testing.T) {
	req := httptest.NewRequest("GET", "http://inner.example/ocpi/2.2.1/sender/sessions?offset=0&limit=10", nil)
	req.Host = "inner.example"
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Header.Set("X-Forwarded-Host", "x-forwarded.example")
	req.Header.Set("X-Rally-Forwarded-Host", "x-rally.example")

	linkHeader := BuildLinkHeader(req, Paging{Offset: 0, Limit: 10}, 10, 30)
	if linkHeader == nil {
		t.Fatal("expected a Link header but got nil")
	}
	got := linkHeader.Get("Link")
	want := `<https://x-rally.example/ocpi/2.2.1/sender/sessions?limit=10&offset=10>; rel="next"`
	if got != want {
		t.Fatalf("unexpected Link header, want %q got %q", want, got)
	}
}

