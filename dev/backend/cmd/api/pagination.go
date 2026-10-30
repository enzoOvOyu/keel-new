package main

import (
	"encoding/base64"
	"encoding/json"
	"github.com/google/uuid"
	"net/http"
	"strconv"
)

func pageBounds(r *http.Request) (int, int, bool) {
	limit := 50
	if q := r.URL.Query().Get("limit"); q != "" {
		n, e := strconv.Atoi(q)
		if e != nil || n < 1 || n > 200 {
			return 0, 0, false
		}
		limit = n
	}
	offset := 0
	if cursor := r.URL.Query().Get("cursor"); cursor != "" {
		b, e := base64.RawURLEncoding.DecodeString(cursor)
		if e != nil {
			return 0, 0, false
		}
		n, e := strconv.Atoi(string(b))
		if e != nil || n < 0 || n > 1000000 {
			return 0, 0, false
		}
		offset = n
	}
	return limit, offset, true
}
func listResponse(w http.ResponseWriter, r *http.Request, rows []any) {
	limit, offset, ok := pageBounds(r)
	if !ok {
		fail(w, 400, "INVALID_PAGE")
		return
	}
	if offset > len(rows) {
		offset = len(rows)
	}
	end := offset + limit
	if end > len(rows) {
		end = len(rows)
	}
	more := end < len(rows)
	var next any
	if more {
		next = base64.RawURLEncoding.EncodeToString([]byte(strconv.Itoa(end)))
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"data": rows[offset:end], "page": map[string]any{"limit": limit, "next_cursor": next, "has_more": more}, "request_id": uuid.NewString()})
}
func pageResponse(w http.ResponseWriter, rows any, limit, offset, count int, more bool) {
	var next any
	if more {
		next = base64.RawURLEncoding.EncodeToString([]byte(strconv.Itoa(offset + count)))
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"data": rows, "page": map[string]any{"limit": limit, "has_more": more, "next_cursor": next}, "request_id": uuid.NewString()})
}
