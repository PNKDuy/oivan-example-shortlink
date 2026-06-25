package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/phngkhuongduy/shortlink/internal/repository"
	"github.com/phngkhuongduy/shortlink/internal/shortener"
)

func newTestServer(t *testing.T) *Handler {
	t.Helper()
	gin.SetMode(gin.TestMode)
	repo := repository.NewMemory()
	t.Cleanup(func() { _ = repo.Close() })
	svc := shortener.NewService(repo, shortener.NewRandomGenerator(7), 5)
	return NewHandler(svc, "http://localhost:8080")
}

func doJSON(t *testing.T, h *Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode body: %v", err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.Router().ServeHTTP(rec, req)
	return rec
}

func TestEncodeDecode_HTTPRoundTrip(t *testing.T) {
	h := newTestServer(t)
	const long = "https://codesubmit.io/library/react"

	rec := doJSON(t, h, http.MethodPost, "/encode", map[string]string{"url": long})
	if rec.Code != http.StatusOK {
		t.Fatalf("encode status = %d, body=%s", rec.Code, rec.Body)
	}
	var enc encodeResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &enc); err != nil {
		t.Fatalf("decode encode resp: %v", err)
	}
	if enc.Code == "" || enc.ShortURL != "http://localhost:8080/"+enc.Code {
		t.Fatalf("unexpected encode response: %+v", enc)
	}

	rec = doJSON(t, h, http.MethodPost, "/decode", map[string]string{"short_url": enc.ShortURL})
	if rec.Code != http.StatusOK {
		t.Fatalf("decode status = %d, body=%s", rec.Code, rec.Body)
	}
	var dec decodeResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &dec); err != nil {
		t.Fatalf("decode resp: %v", err)
	}
	if dec.URL != long {
		t.Fatalf("decoded url = %q, want %q", dec.URL, long)
	}
}

func TestEncode_BadRequests(t *testing.T) {
	h := newTestServer(t)
	cases := []struct {
		name string
		body any
		raw  string // used when body is nil
	}{
		{name: "invalid url", body: map[string]string{"url": "not-a-url"}},
		{name: "javascript scheme", body: map[string]string{"url": "javascript:alert(1)"}},
		{name: "empty url", body: map[string]string{"url": ""}},
		{name: "malformed json", raw: "{not json"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var rec *httptest.ResponseRecorder
			if tc.body != nil {
				rec = doJSON(t, h, http.MethodPost, "/encode", tc.body)
			} else {
				req := httptest.NewRequest(http.MethodPost, "/encode", bytes.NewBufferString(tc.raw))
				req.Header.Set("Content-Type", "application/json")
				rec = httptest.NewRecorder()
				h.Router().ServeHTTP(rec, req)
			}
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 (body=%s)", rec.Code, rec.Body)
			}
		})
	}
}

func TestDecode_NotFound(t *testing.T) {
	h := newTestServer(t)
	rec := doJSON(t, h, http.MethodPost, "/decode", map[string]string{"short_url": "http://localhost:8080/missing"})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (body=%s)", rec.Code, rec.Body)
	}
}

func TestRedirect(t *testing.T) {
	h := newTestServer(t)
	const long = "https://example.com/target"
	rec := doJSON(t, h, http.MethodPost, "/encode", map[string]string{"url": long})
	var enc encodeResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &enc)

	req := httptest.NewRequest(http.MethodGet, "/"+enc.Code, nil)
	rec = httptest.NewRecorder()
	h.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusFound {
		t.Fatalf("redirect status = %d, want 302", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != long {
		t.Fatalf("Location = %q, want %q", loc, long)
	}
}

func TestHealth(t *testing.T) {
	h := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	h.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("health status = %d, want 200", rec.Code)
	}
}
