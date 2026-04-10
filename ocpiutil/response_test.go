package ocpiutil

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestParseDateRange(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		wantFrom bool
		wantTo   bool
	}{
		{"both set", "date_from=2026-01-01T00:00:00Z&date_to=2026-06-01T00:00:00Z", true, true},
		{"from only", "date_from=2026-01-01T00:00:00Z", true, false},
		{"to only", "date_to=2026-06-01T00:00:00Z", false, true},
		{"neither", "", false, false},
		{"invalid from", "date_from=bad&date_to=2026-06-01T00:00:00Z", false, true},
		{"invalid both", "date_from=bad&date_to=bad", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/test?"+tt.query, nil)
			from, to := ParseDateRange(r)
			if (from != nil) != tt.wantFrom {
				t.Errorf("from: got %v, want present=%v", from, tt.wantFrom)
			}
			if (to != nil) != tt.wantTo {
				t.Errorf("to: got %v, want present=%v", to, tt.wantTo)
			}
		})
	}
}

func TestFilterByLastUpdated(t *testing.T) {
	type item struct {
		Name        string
		LastUpdated string
	}

	items := []item{
		{"a", "2026-01-01T00:00:00Z"},
		{"b", "2026-03-01T00:00:00Z"},
		{"c", "2026-06-01T00:00:00Z"},
		{"d", "2026-09-01T00:00:00Z"},
	}
	accessor := func(i item) string { return i.LastUpdated }

	t.Run("no filter", func(t *testing.T) {
		result := FilterByLastUpdated(items, accessor, nil, nil)
		if len(result) != 4 {
			t.Errorf("expected 4, got %d", len(result))
		}
	})

	t.Run("from only", func(t *testing.T) {
		from := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
		result := FilterByLastUpdated(items, accessor, &from, nil)
		if len(result) != 3 {
			t.Errorf("expected 3, got %d", len(result))
		}
	})

	t.Run("to only", func(t *testing.T) {
		to := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
		result := FilterByLastUpdated(items, accessor, nil, &to)
		if len(result) != 2 {
			t.Errorf("expected 2, got %d", len(result))
		}
	})

	t.Run("both", func(t *testing.T) {
		from := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
		to := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
		result := FilterByLastUpdated(items, accessor, &from, &to)
		if len(result) != 2 {
			t.Errorf("expected 2 (b,c), got %d", len(result))
		}
	})
}

func TestFilterRawByLastUpdated(t *testing.T) {
	items := [][]byte{
		[]byte(`{"last_updated":"2026-01-01T00:00:00Z","id":"1"}`),
		[]byte(`{"last_updated":"2026-06-01T00:00:00Z","id":"2"}`),
		[]byte(`{"last_updated":"2026-12-01T00:00:00Z","id":"3"}`),
	}

	from := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	result := FilterRawByLastUpdated(items, &from, nil)
	if len(result) != 2 {
		t.Errorf("expected 2, got %d", len(result))
	}
}

func TestBuildPagingHeaders(t *testing.T) {
	t.Run("middle page", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/test?offset=10&limit=10", nil)
		p := Paging{Offset: 10, Limit: 10}
		h := BuildPagingHeaders(r, p, 10, 30)

		if h.Get("X-Total-Count") != "30" {
			t.Errorf("X-Total-Count: got %s, want 30", h.Get("X-Total-Count"))
		}
		if h.Get("X-Limit") != "10" {
			t.Errorf("X-Limit: got %s, want 10", h.Get("X-Limit"))
		}
		if h.Get("Link") == "" {
			t.Error("expected Link header for middle page")
		}
	})

	t.Run("last page", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/test?offset=20&limit=10", nil)
		p := Paging{Offset: 20, Limit: 10}
		h := BuildPagingHeaders(r, p, 5, 25)

		if h.Get("X-Total-Count") != "25" {
			t.Errorf("X-Total-Count: got %s, want 25", h.Get("X-Total-Count"))
		}
		if h.Get("Link") != "" {
			t.Errorf("expected no Link header on last page, got %s", h.Get("Link"))
		}
	})

	t.Run("single page", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/test", nil)
		p := Paging{Offset: 0, Limit: 50}
		h := BuildPagingHeaders(r, p, 3, 3)

		if h.Get("X-Total-Count") != "3" {
			t.Errorf("X-Total-Count: got %s, want 3", h.Get("X-Total-Count"))
		}
		if h.Get("Link") != "" {
			t.Error("expected no Link header on single page")
		}
	})
}

func TestBuildPagingHeadersPrefersXRallyForwardedHost(t *testing.T) {
	req := httptest.NewRequest("GET", "http://inner.example/ocpi/2.2.1/sender/sessions?offset=0&limit=10", nil)
	req.Host = "inner.example"
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Header.Set("X-Forwarded-Host", "x-forwarded.example")
	req.Header.Set("X-Rally-Forwarded-Host", "x-rally.example")

	h := BuildPagingHeaders(req, Paging{Offset: 0, Limit: 10}, 10, 30)
	got := h.Get("Link")
	want := `<https://x-rally.example/ocpi/2.2.1/sender/sessions?limit=10&offset=10>; rel="next"`
	if got != want {
		t.Fatalf("unexpected Link header, want %q got %q", want, got)
	}
}

func TestParsePaging(t *testing.T) {
	r := httptest.NewRequest("GET", "/test?offset=5&limit=20", nil)
	p := ParsePaging(r, 50)
	if p.Offset != 5 || p.Limit != 20 {
		t.Errorf("got offset=%d limit=%d, want 5 and 20", p.Offset, p.Limit)
	}

	r2 := httptest.NewRequest("GET", "/test", nil)
	p2 := ParsePaging(r2, 50)
	if p2.Offset != 0 || p2.Limit != 50 {
		t.Errorf("got offset=%d limit=%d, want 0 and 50", p2.Offset, p2.Limit)
	}
}

func TestPaginateSlice(t *testing.T) {
	items := []int{1, 2, 3, 4, 5}

	page := PaginateSlice(items, Paging{Offset: 0, Limit: 3})
	if len(page) != 3 || page[0] != 1 {
		t.Errorf("page 1: got %v, want [1,2,3]", page)
	}

	page = PaginateSlice(items, Paging{Offset: 3, Limit: 3})
	if len(page) != 2 || page[0] != 4 {
		t.Errorf("page 2: got %v, want [4,5]", page)
	}

	page = PaginateSlice(items, Paging{Offset: 10, Limit: 3})
	if page != nil {
		t.Errorf("past end: got %v, want nil", page)
	}
}

func TestOK_Response(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)

	OK(w, r, map[string]string{"key": "val"})

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status: got %d, want 200", resp.StatusCode)
	}
	if resp.Header.Get("Content-Type") != "application/json" {
		t.Errorf("content-type: got %s", resp.Header.Get("Content-Type"))
	}
	if resp.Header.Get("X-Request-ID") == "" {
		t.Error("expected X-Request-ID header")
	}
}
