package apitest_test

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"

	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/platform/apitest"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/platform/httpserver"
)

// handler serves the API of testdata/things.openapi.yaml.
func handler(t *testing.T) (*openapi3.T, http.Handler) {
	t.Helper()
	doc, err := openapi3.NewLoader().LoadFromFile("testdata/things.openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	h, err := httpserver.NewHandler(doc, slog.New(slog.DiscardHandler), func(mux *http.ServeMux) {
		mux.HandleFunc("GET /things/lamp", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"name":"lamp"}`))
		})
	})
	if err != nil {
		t.Fatal(err)
	}
	return doc, h
}

func TestInMemoryAndRemoteClientsAgree(t *testing.T) {
	doc, h := handler(t)
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	clients := map[string]*apitest.Client{
		"in memory": apitest.New(t, doc, h),
		"remote":    apitest.NewRemote(t, doc, srv.URL+"/"),
	}
	for name, client := range clients {
		res := client.Do(http.MethodGet, "/things/lamp", nil)
		var thing struct {
			Name string `json:"name"`
		}
		res.JSON(t, &thing)
		if res.Status != http.StatusOK || thing.Name != "lamp" {
			t.Errorf("%s: got %d %+v, want 200 with the name lamp", name, res.Status, thing)
		}
	}
}
