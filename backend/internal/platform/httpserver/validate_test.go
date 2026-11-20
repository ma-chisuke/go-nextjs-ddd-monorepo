package httpserver

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"

	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/platform/problem"
)

// countsHandler returns the handler of the API of testdata/counts.openapi.yaml,
// whose operation decodes its body as the generated code does, and records
// what it read.
func countsHandler(t *testing.T, seen *[]byte) http.Handler {
	t.Helper()
	doc, err := openapi3.NewLoader().LoadFromFile("testdata/counts.openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	handler, err := NewHandler(doc, slog.New(slog.DiscardHandler), func(mux *http.ServeMux) {
		mux.HandleFunc("GET /counts", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
		mux.HandleFunc("POST /counts", func(w http.ResponseWriter, r *http.Request) {
			raw, err := io.ReadAll(r.Body)
			if err != nil {
				t.Errorf("the handler cannot read the body: %v", err)
			}
			*seen = raw
			var body struct {
				Count int32   `json:"count"`
				More  []int64 `json:"more"`
			}
			if err := json.Unmarshal(raw, &body); err != nil {
				RequestError(w, r, err)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		})
	})
	if err != nil {
		t.Fatal(err)
	}
	return handler
}

func serve(t *testing.T, handler http.Handler, method, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), method, "/counts", strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func TestMethodNotAllowedNamesTheMethodsOfTheURL(t *testing.T) {
	var seen []byte
	handler := countsHandler(t, &seen)
	for _, method := range []string{http.MethodDelete, http.MethodTrace, http.MethodOptions} {
		rec := serve(t, handler, method, "")
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s /counts: got status %d, want 405", method, rec.Code)
		}
		if got, want := rec.Header().Get("Allow"), "GET, POST"; got != want {
			t.Errorf("%s /counts: got Allow %q, want %q", method, got, want)
		}
		if got := rec.Header().Get("Content-Type"); got != problem.MediaType {
			t.Errorf("%s /counts: got the content type %q, want a problem", method, got)
		}
	}
}

func TestWholeNumbersReachTheHandlerAsIntegers(t *testing.T) {
	tests := []struct {
		name   string
		body   string
		status int
		// seen is what the handler reads; empty when it is the body as sent.
		seen string
	}{
		{"an integer", `{"count": 4}`, http.StatusNoContent, ""},
		{"a whole number with a fraction", `{"count": 4.0}`, http.StatusNoContent, `{"count":4}`},
		{"a whole number with an exponent", `{"count": 1e3, "more": [2.50e1, 7]}`, http.StatusNoContent, `{"count":1000,"more":[25,7]}`},
		{"a fraction where the document allows numbers", `{"count": 4, "ratio": 0.5, "label": "4.0"}`, http.StatusNoContent, ""},
		{"a fraction where the document asks for an integer", `{"count": 4.5}`, http.StatusUnprocessableEntity, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var seen []byte
			rec := serve(t, countsHandler(t, &seen), http.MethodPost, tt.body)
			if rec.Code != tt.status {
				t.Fatalf("got status %d, want %d\n%s", rec.Code, tt.status, rec.Body)
			}
			if tt.status != http.StatusNoContent {
				return
			}
			want := tt.seen
			if want == "" {
				want = tt.body
			}
			if !bytes.Equal(seen, []byte(want)) {
				t.Errorf("the handler read %s, want %s", seen, want)
			}
		})
	}
}
