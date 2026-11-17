package httpserver

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/gorillamux"

	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/platform/problem"
)

var registerFormats sync.Once

// RegisterFormats makes the validator know the string formats and the media
// type that the API documents of this repository use. kin-openapi keeps them
// in package-level tables, so this runs once per process.
func RegisterFormats() {
	registerFormats.Do(func() {
		openapi3.DefineStringFormatValidator("uuid", openapi3.NewRegexpFormatValidator(
			`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`))
		// A URI reference can be almost any string; the format documents intent.
		openapi3.DefineStringFormatValidator("uri-reference", openapi3.NewCallbackValidator(func(string) error { return nil }))
		openapi3filter.RegisterBodyDecoder(problem.MediaType, openapi3filter.JSONBodyDecoder)
	})
}

// ValidateRequests returns a middleware that checks every request against the
// API document before a handler sees it: the operation exists, the parameters
// and the body have the documented shape. A request that fails gets a problem
// (TPL-0008); production validates requests only, and tests also validate
// responses (TPL-0007).
func ValidateRequests(doc *openapi3.T) (func(http.Handler) http.Handler, error) {
	RegisterFormats()
	router, err := gorillamux.NewRouter(doc)
	if err != nil {
		return nil, err
	}
	options := &openapi3filter.Options{
		MultiError:         true,
		AuthenticationFunc: openapi3filter.NoopAuthenticationFunc,
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			route, pathParams, err := router.FindRoute(r)
			if err != nil {
				problem.Write(w, routeProblem(err))
				return
			}
			input := &openapi3filter.RequestValidationInput{
				Request:    r,
				PathParams: pathParams,
				Route:      route,
				Options:    options,
			}
			if err := openapi3filter.ValidateRequest(r.Context(), input); err != nil {
				problem.Write(w, requestProblem(r.Context(), err))
				return
			}
			next.ServeHTTP(w, r)
		})
	}, nil
}

func routeProblem(err error) *problem.Problem {
	if errors.Is(err, routers.ErrMethodNotAllowed) {
		return problem.New(problem.MethodNotAllowed, "")
	}
	return problem.New(problem.NotFound, "")
}

// requestProblem turns the errors of the validator into one problem. A body
// or a parameter that cannot be read is a malformed request (400); a value
// that can be read and breaks a rule of the document is a validation error
// (422), with a pointer to each such value.
func requestProblem(_ context.Context, err error) *problem.Problem {
	var invalid []problem.InvalidParam
	for _, e := range flatten(err) {
		var re *openapi3filter.RequestError
		if !errors.As(e, &re) {
			return problem.New(problem.MalformedRequest, e.Error())
		}
		var pe *openapi3filter.ParseError
		if errors.As(re.Err, &pe) {
			return problem.New(problem.MalformedRequest, where(re)+" cannot be read: "+pe.Error())
		}
		if re.RequestBody != nil && strings.Contains(re.Reason, "Content-Type") {
			return problem.New(problem.UnsupportedMediaType, re.Reason)
		}
		invalid = append(invalid, invalidParams(re)...)
	}
	return problem.Invalid("The request does not match the API document.", invalid...)
}

// flatten returns the errors that err holds when err is itself a list of
// errors. It does not look inside a wrapped error: a RequestError wraps a
// list of schema errors, and must stay one error here.
func flatten(err error) []error {
	multi, ok := err.(openapi3.MultiError) //nolint:errorlint // See the comment above.
	if !ok {
		return []error{err}
	}
	var all []error
	for _, e := range multi {
		all = append(all, flatten(e)...)
	}
	return all
}

func where(re *openapi3filter.RequestError) string {
	if re.Parameter != nil {
		return "The parameter " + re.Parameter.Name
	}
	return "The request body"
}

func invalidParams(re *openapi3filter.RequestError) []problem.InvalidParam {
	base := ""
	if re.Parameter != nil {
		base = re.Parameter.Name
	}
	var params []problem.InvalidParam
	for _, e := range flatten(re.Err) {
		var se *openapi3.SchemaError
		if errors.As(e, &se) {
			params = append(params, problem.InvalidParam{
				Pointer: base + pointer(se.JSONPointer()),
				Detail:  se.Reason,
			})
			continue
		}
		detail := re.Reason
		if e != nil {
			detail = e.Error()
		}
		params = append(params, problem.InvalidParam{Pointer: base, Detail: detail})
	}
	return params
}

// pointer renders the path of a schema error as a JSON Pointer (RFC 6901).
func pointer(path []string) string {
	var b strings.Builder
	for _, token := range path {
		b.WriteByte('/')
		b.WriteString(strings.NewReplacer("~", "~0", "/", "~1").Replace(token))
	}
	return b.String()
}
