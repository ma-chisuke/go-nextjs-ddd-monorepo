// Package problem writes the errors of an HTTP API as problem details
// (RFC 9457, application/problem+json). See docs/adr/template/TPL-0008.
package problem

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

// MediaType is the media type of a problem.
const MediaType = "application/problem+json"

// docsBase is where the page of each kind of problem lives. The URI of a kind
// is docsBase + its slug + ".md", so a client that follows the URI reads what
// the problem means and what to do about it.
const docsBase = "https://github.com/ma-chisuke/go-nextjs-ddd-monorepo/blob/main/docs/problems/"

// Type is one kind of problem. Every Type has a page docs/problems/<Slug>.md.
type Type struct {
	// Slug names the kind in the URI and in the file name of its page.
	Slug string
	// Title is the same for every occurrence of the kind.
	Title string
	// Status is the HTTP status code.
	Status int
}

// URI returns the URI that identifies the kind and leads to its page.
func (t Type) URI() string {
	return docsBase + t.Slug + ".md"
}

// The kinds of problem that every API of this repository can answer with.
var (
	// MalformedRequest: the request cannot be read, for example broken JSON.
	MalformedRequest = Type{Slug: "malformed-request", Title: "The request cannot be read", Status: http.StatusBadRequest}
	// NotFound: no operation or resource answers to the URL.
	NotFound = Type{Slug: "not-found", Title: "Nothing is at this URL", Status: http.StatusNotFound}
	// MethodNotAllowed: the URL exists, and the method is not one of its operations.
	MethodNotAllowed = Type{Slug: "method-not-allowed", Title: "The method is not allowed for this URL", Status: http.StatusMethodNotAllowed}
	// PreconditionFailed: the If-Match header names a version that is not the current one.
	PreconditionFailed = Type{Slug: "precondition-failed", Title: "The resource changed since it was read", Status: http.StatusPreconditionFailed}
	// UnsupportedMediaType: the body is in a media type that the operation does not take.
	UnsupportedMediaType = Type{Slug: "unsupported-media-type", Title: "The media type of the body is not supported", Status: http.StatusUnsupportedMediaType}
	// ValidationError: the request can be read, and a part of it breaks a rule of the API document.
	ValidationError = Type{Slug: "validation-error", Title: "The request is not valid", Status: http.StatusUnprocessableEntity}
	// PreconditionRequired: the operation needs an If-Match header.
	PreconditionRequired = Type{Slug: "precondition-required", Title: "The request needs an If-Match header", Status: http.StatusPreconditionRequired}
	// InternalError: the service failed. The body says nothing about the cause.
	InternalError = Type{Slug: "internal-error", Title: "The service failed", Status: http.StatusInternalServerError}
)

// CommonTypes lists the kinds above, for the test that every kind has a page.
func CommonTypes() []Type {
	return []Type{
		MalformedRequest, NotFound, MethodNotAllowed, PreconditionFailed,
		UnsupportedMediaType, ValidationError, PreconditionRequired, InternalError,
	}
}

// InvalidParam points at one part of a request that is not valid.
type InvalidParam struct {
	// Pointer is a JSON Pointer (RFC 6901) into the request body, or the name
	// of a parameter.
	Pointer string `json:"pointer"`
	// Detail says why the part is not valid.
	Detail string `json:"detail"`
}

// Problem is one occurrence of a problem, in the shape of RFC 9457.
type Problem struct {
	Type     string         `json:"type"`
	Title    string         `json:"title"`
	Status   int            `json:"status"`
	Detail   string         `json:"detail,omitempty"`
	Instance string         `json:"instance,omitempty"`
	Errors   []InvalidParam `json:"errors,omitempty"`
}

// New returns an occurrence of the kind t with the given detail.
func New(t Type, detail string) *Problem {
	return &Problem{Type: t.URI(), Title: t.Title, Status: t.Status, Detail: detail}
}

// Invalid returns a validation problem that points at the invalid parts.
func Invalid(detail string, params ...InvalidParam) *Problem {
	p := New(ValidationError, detail)
	p.Errors = params
	return p
}

// Error makes a Problem usable as an error, so that a handler can return it
// and the server writes it as the response.
func (p *Problem) Error() string {
	return fmt.Sprintf("problem %d %s: %s", p.Status, p.Title, p.Detail)
}

// From returns the problem that err is or wraps. Any other error becomes an
// internal error that reveals nothing about its cause; ok is false then, and
// the caller logs err.
func From(err error) (p *Problem, ok bool) {
	if errors.As(err, &p) {
		return p, true
	}
	return New(InternalError, ""), false
}

// Write writes p as the response.
func Write(w http.ResponseWriter, p *Problem) {
	body, err := json.Marshal(p)
	if err != nil {
		// A Problem holds only strings and an int, so this cannot happen.
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", MediaType)
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(p.Status)
	_, _ = w.Write(body)
}
