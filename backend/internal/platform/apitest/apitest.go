// Package apitest is the HTTP client of the API tests. It checks every
// request and every response against the API document, so a test fails when
// the implementation and the document disagree in a way that Go types cannot
// express: a format, a limit, a required property, an undocumented status
// code, or the shape of a problem (TPL-0007, layer 4).
package apitest

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/gorillamux"

	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/platform/httpserver"
)

// Client sends requests to a handler in memory.
type Client struct {
	t       *testing.T
	handler http.Handler
	router  routers.Router
}

// New returns a client for handler, which serves the API that doc describes.
func New(t *testing.T, doc *openapi3.T, handler http.Handler) *Client {
	t.Helper()
	httpserver.RegisterFormats()
	router, err := gorillamux.NewRouter(doc)
	if err != nil {
		t.Fatalf("apitest: the API document has no usable routes: %v", err)
	}
	return &Client{t: t, handler: handler, router: router}
}

// Response is what the handler answered.
type Response struct {
	Status int
	Header http.Header
	Body   []byte
}

// JSON decodes the body into v.
func (r Response) JSON(t *testing.T, v any) {
	t.Helper()
	if err := json.Unmarshal(r.Body, v); err != nil {
		t.Fatalf("apitest: the body is not the expected JSON: %v\n%s", err, r.Body)
	}
}

// Raw sends a request without a body to handler and checks nothing. It is
// for the endpoints that are not operations of the API, such as the health
// check.
func Raw(handler http.Handler, method, target string) Response {
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), method, target, nil))
	return Response{Status: rec.Code, Header: rec.Header(), Body: rec.Body.Bytes()}
}

// Option changes a request before it is sent.
type Option func(*http.Request)

// Header sets a request header.
func Header(name, value string) Option {
	return func(r *http.Request) { r.Header.Set(name, value) }
}

// Do sends a request whose body is the JSON form of body (nil for no body),
// and fails the test when the response does not match the API document.
//
// The request is not checked here, because tests also send invalid requests
// on purpose; the validation middleware of the server answers those.
func (c *Client) Do(method, target string, body any, options ...Option) Response {
	c.t.Helper()
	var reader io.Reader
	if body != nil {
		raw, ok := body.([]byte)
		if !ok {
			var err error
			if raw, err = json.Marshal(body); err != nil {
				c.t.Fatalf("apitest: cannot encode the request body: %v", err)
			}
		}
		reader = bytes.NewReader(raw)
	}
	req := httptest.NewRequestWithContext(context.Background(), method, target, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for _, option := range options {
		option(req)
	}
	rec := httptest.NewRecorder()
	c.handler.ServeHTTP(rec, req)
	res := Response{Status: rec.Code, Header: rec.Header(), Body: rec.Body.Bytes()}
	c.validate(req, res)
	return res
}

// validate checks the response against the operation that the request
// addressed. A request that addresses no operation has no documented
// responses; its answer must still be a problem.
func (c *Client) validate(req *http.Request, res Response) {
	c.t.Helper()
	route, pathParams, err := c.router.FindRoute(req)
	if err != nil {
		if ct := res.Header.Get("Content-Type"); res.Status < 400 || ct != "application/problem+json" {
			c.t.Errorf("apitest: %s %s addresses no operation, and the answer is %d %q, not a problem", req.Method, req.URL.Path, res.Status, ct)
		}
		return
	}
	input := &openapi3filter.ResponseValidationInput{
		RequestValidationInput: &openapi3filter.RequestValidationInput{
			Request:    req,
			PathParams: pathParams,
			Route:      route,
			Options: &openapi3filter.Options{
				AuthenticationFunc: openapi3filter.NoopAuthenticationFunc,
			},
		},
		Status: res.Status,
		Header: res.Header,
		Body:   io.NopCloser(bytes.NewReader(res.Body)),
		Options: &openapi3filter.Options{
			IncludeResponseStatus: true,
			MultiError:            true,
		},
	}
	if err := openapi3filter.ValidateResponse(context.Background(), input); err != nil {
		c.t.Errorf("apitest: the response of %s %s (%d) does not match the API document: %v\n%s", req.Method, req.URL.Path, res.Status, err, res.Body)
	}
}
