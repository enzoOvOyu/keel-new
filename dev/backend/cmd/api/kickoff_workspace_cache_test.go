package main

import (
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWorkspaceCompression(t *testing.T) {
	for _, encoding := range []string{"gzip, deflate, br", "gzip;q=0", ""} {
		r := httptest.NewRequest("GET", "/workspace", nil)
		r.Header.Set("Accept-Encoding", encoding)
		w := httptest.NewRecorder()
		respondKickoffWorkspace(w, r, map[string]string{"value": strings.Repeat("engineering", 100)})
		raw := w.Body.Bytes()
		if encoding == "gzip, deflate, br" {
			if w.Header().Get("Content-Encoding") != "gzip" {
				t.Fatal("compression missing")
			}
			z, err := gzip.NewReader(w.Body)
			if err != nil {
				t.Fatal(err)
			}
			raw, err = io.ReadAll(z)
			z.Close()
			if err != nil {
				t.Fatal(err)
			}
		} else if w.Header().Get("Content-Encoding") != "" {
			t.Fatal("client rejected gzip")
		}
		var result struct {
			Data map[string]string `json:"data"`
		}
		if json.Unmarshal(raw, &result) != nil || result.Data["value"] != strings.Repeat("engineering", 100) {
			t.Fatal("response contract changed")
		}
	}
}
