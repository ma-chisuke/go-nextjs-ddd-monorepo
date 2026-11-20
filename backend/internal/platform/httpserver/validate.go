package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"mime"
	"net/http"
	"strconv"
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
// responses (TPL-0007). A JSON body that passes reaches the handler with its
// whole numbers written as integers (wholeNumbersAsIntegers).
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
			if errors.Is(err, routers.ErrMethodNotAllowed) {
				// A 405 names the methods that the URL has (RFC 9110,
				// section 15.5.6).
				w.Header().Set("Allow", strings.Join(allowedMethods(router, r), ", "))
				problem.Write(w, problem.New(problem.MethodNotAllowed, ""))
				return
			}
			if err != nil {
				problem.Write(w, problem.New(problem.NotFound, ""))
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
			if err := wholeNumbersAsIntegers(r); err != nil {
				problem.Write(w, problem.New(problem.MalformedRequest, "The request body cannot be read: "+err.Error()))
				return
			}
			next.ServeHTTP(w, r)
		})
	}, nil
}

// methods are the methods that an operation of an API document can have.
var methods = []string{
	http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut, http.MethodPatch,
	http.MethodDelete, http.MethodOptions, http.MethodTrace,
}

// allowedMethods returns the methods of the operations at the URL of r.
func allowedMethods(router routers.Router, r *http.Request) []string {
	var allowed []string
	for _, method := range methods {
		probe := r.WithContext(r.Context())
		probe.Method = method
		if _, _, err := router.FindRoute(probe); err == nil {
			allowed = append(allowed, method)
		}
	}
	return allowed
}

// largestExactInteger is 2^53: every integer up to it in magnitude has an
// exact float64, so the validator, which reads numbers as float64, and the
// handler agree on its value.
const largestExactInteger = 1 << 53

// wholeNumbersAsIntegers writes each number of a JSON request body that has a
// fraction or an exponent and still holds a whole number, such as 4.0 or 1e3,
// in the form of an integer, 4 or 1000.
//
// JSON Schema counts such a number as an integer, so the validator accepts it
// where the document asks for an integer. The handler decodes the body with
// encoding/json, which refuses it for an integer type, and would answer that
// the body cannot be read. After the rewrite, the handler reads every body
// that the validator accepted. A body without such a number passes as it is.
func wholeNumbersAsIntegers(r *http.Request) error {
	if r.Body == nil || r.Body == http.NoBody || !isJSON(r.Header.Get("Content-Type")) {
		return nil
	}
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	_ = r.Body.Close()
	setBody(r, raw)
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return err
	}
	value, changed := wholeNumbers(value)
	if !changed {
		return nil
	}
	rewritten, err := json.Marshal(value)
	if err != nil {
		return err
	}
	setBody(r, rewritten)
	return nil
}

// wholeNumbers returns v with its whole numbers written as integers, and
// whether it changed any.
func wholeNumbers(v any) (any, bool) {
	switch v := v.(type) {
	case json.Number:
		s := v.String()
		if !strings.ContainsAny(s, ".eE") {
			return v, false
		}
		f, err := strconv.ParseFloat(s, 64)
		if err != nil || f != math.Trunc(f) || math.Abs(f) > largestExactInteger {
			return v, false
		}
		return json.Number(strconv.FormatInt(int64(f), 10)), true
	case map[string]any:
		changed := false
		for key, item := range v {
			var c bool
			v[key], c = wholeNumbers(item)
			changed = changed || c
		}
		return v, changed
	case []any:
		changed := false
		for i, item := range v {
			var c bool
			v[i], c = wholeNumbers(item)
			changed = changed || c
		}
		return v, changed
	default:
		return v, false
	}
}

// isJSON reports whether a media type is JSON: application/json, or a type
// with the suffix +json.
func isJSON(contentType string) bool {
	mediaType, _, err := mime.ParseMediaType(contentType)
	return err == nil && (mediaType == "application/json" || strings.HasSuffix(mediaType, "+json"))
}

// setBody replaces the body of r with body.
func setBody(r *http.Request, body []byte) {
	r.Body = io.NopCloser(bytes.NewReader(body))
	r.ContentLength = int64(len(body))
	r.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(body)), nil
	}
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
