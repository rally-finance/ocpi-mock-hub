package ocpiutil

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
)

const (
	StatusSuccess       = 1000
	StatusClientError   = 2000
	StatusUnauthorized  = 2001
	StatusInvalidParams = 2003
	StatusUnknownObject = 2004
	StatusServerError   = 3000
)

type Response struct {
	Data          any    `json:"data,omitempty"`
	StatusCode    int    `json:"status_code"`
	StatusMessage string `json:"status_message,omitempty"`
	Timestamp     string `json:"timestamp"`
}

func OK(w http.ResponseWriter, r *http.Request, data any, extra ...http.Header) {
	resp := Response{
		Data:       data,
		StatusCode: StatusSuccess,
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
	}
	Write(w, r, http.StatusOK, resp, extra...)
}

func Error(w http.ResponseWriter, r *http.Request, httpStatus int, ocpiCode int, msg string) {
	resp := Response{
		StatusCode:    ocpiCode,
		StatusMessage: msg,
		Timestamp:     time.Now().UTC().Format(time.RFC3339),
	}
	Write(w, r, httpStatus, resp)
}

func Write(w http.ResponseWriter, r *http.Request, httpStatus int, resp Response, extra ...http.Header) {
	SetHeaders(w, r)
	for _, h := range extra {
		for k, vv := range h {
			for _, v := range vv {
				w.Header().Set(k, v)
			}
		}
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatus)
	json.NewEncoder(w).Encode(resp)
}

func SetHeaders(w http.ResponseWriter, r *http.Request) {
	reqID := r.Header.Get("X-Request-ID")
	if reqID == "" {
		reqID = uuid.NewString()
	}
	corrID := r.Header.Get("X-Correlation-ID")
	if corrID == "" {
		corrID = uuid.NewString()
	}
	w.Header().Set("X-Request-ID", reqID)
	w.Header().Set("X-Correlation-ID", corrID)
	w.Header().Set("OCPI-Correlation-ID", corrID)
}

type Paging struct {
	Offset int
	Limit  int
}

func ParsePaging(r *http.Request, defaultLimit int) Paging {
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 {
		limit = defaultLimit
	}
	if limit > 250 {
		limit = 250
	}
	return Paging{Offset: offset, Limit: limit}
}

func BuildPagingHeaders(r *http.Request, p Paging, returned, total int) http.Header {
	h := http.Header{}
	h.Set("X-Total-Count", strconv.Itoa(total))
	h.Set("X-Limit", strconv.Itoa(p.Limit))

	if returned >= p.Limit && p.Offset+returned < total {
		nextOffset := p.Offset + p.Limit
		u := *r.URL
		q := u.Query()
		q.Set("offset", strconv.Itoa(nextOffset))
		q.Set("limit", strconv.Itoa(p.Limit))
		u.RawQuery = q.Encode()

		scheme := "https"
		if r.TLS == nil {
			if fwd := r.Header.Get("X-Forwarded-Proto"); fwd != "" {
				scheme = fwd
			} else {
				scheme = "http"
			}
		}
		host := r.Header.Get("X-Rally-Forwarded-Host")
		if host == "" {
			host = r.Header.Get("X-Forwarded-Host")
		}
		if host == "" {
			host = r.Host
		}

		link := fmt.Sprintf("<%s://%s%s>; rel=\"next\"", scheme, host, u.RequestURI())
		h.Set("Link", link)
	}

	return h
}

func ParseDateRange(r *http.Request) (from, to *time.Time) {
	if v := r.URL.Query().Get("date_from"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			from = &t
		}
	}
	if v := r.URL.Query().Get("date_to"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			to = &t
		}
	}
	return
}

func FilterByLastUpdated[T any](items []T, getLastUpdated func(T) string, from, to *time.Time) []T {
	if from == nil && to == nil {
		return items
	}
	result := make([]T, 0, len(items))
	for _, item := range items {
		t, err := time.Parse(time.RFC3339, getLastUpdated(item))
		if err != nil {
			continue
		}
		if from != nil && t.Before(*from) {
			continue
		}
		if to != nil && !t.Before(*to) {
			continue
		}
		result = append(result, item)
	}
	return result
}

func FilterRawByLastUpdated(items [][]byte, from, to *time.Time) [][]byte {
	if from == nil && to == nil {
		return items
	}
	type ts struct {
		LastUpdated string `json:"last_updated"`
	}
	result := make([][]byte, 0, len(items))
	for _, b := range items {
		var v ts
		if err := json.Unmarshal(b, &v); err != nil {
			continue
		}
		t, err := time.Parse(time.RFC3339, v.LastUpdated)
		if err != nil {
			continue
		}
		if from != nil && t.Before(*from) {
			continue
		}
		if to != nil && !t.Before(*to) {
			continue
		}
		result = append(result, b)
	}
	return result
}

func PaginateSlice[T any](items []T, p Paging) []T {
	if p.Offset >= len(items) {
		return nil
	}
	end := p.Offset + p.Limit
	if end > len(items) {
		end = len(items)
	}
	return items[p.Offset:end]
}
