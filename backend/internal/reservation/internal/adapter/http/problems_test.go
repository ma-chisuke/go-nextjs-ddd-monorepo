package http

import (
	"testing"

	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/platform/problem"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/platform/problem/problemtest"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/reservation/internal/app"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/reservation/internal/domain"
)

// Every business error has a problem of its own, and every problem has a page.
func TestEveryBusinessErrorHasAProblem(t *testing.T) {
	t.Parallel()
	var kinds []problem.Type
	for _, kind := range domain.Kinds() {
		got := domainProblem(kind)
		if got == problem.InternalError {
			t.Errorf("the rule of the domain of kind %d has no problem: add it to domainProblem", kind)
			continue
		}
		kinds = append(kinds, got)
	}
	for _, kind := range app.Kinds() {
		got := appProblem(kind)
		if got == problem.InternalError {
			t.Errorf("the error of the use cases of kind %d has no problem: add it to appProblem", kind)
			continue
		}
		kinds = append(kinds, got)
	}
	// The operations answer with this one themselves.
	kinds = append(kinds, problem.PreconditionRequired)
	problemtest.AssertDocumented(t, kinds...)
}

func TestIfMatch(t *testing.T) {
	t.Parallel()
	text := func(s string) *string { return &s }
	tests := []struct {
		header *string
		want   app.Version
		slug   string // empty: the header names the version want
	}{
		{nil, 0, "precondition-required"},
		{text("*"), 0, "precondition-required"},
		{text(`"3"`), 3, ""},
		{text(` "12" `), 12, ""},
		{text(`W/"3"`), 0, "precondition-failed"},
		{text(`3`), 0, "precondition-failed"},
		{text(`"03"`), 0, "precondition-failed"},
		{text(`"+3"`), 0, "precondition-failed"},
		{text(`"0"`), 0, "precondition-failed"},
		{text(`"-1"`), 0, "precondition-failed"},
		{text(`"3", "4"`), 0, "precondition-failed"},
		{text(`"`), 0, "precondition-failed"},
	}
	for _, tt := range tests {
		got, err := ifMatch(tt.header)
		name := "no header"
		if tt.header != nil {
			name = *tt.header
		}
		if tt.slug == "" {
			if err != nil || got != tt.want {
				t.Errorf("%s: got %d, %v, want the version %d", name, got, err, tt.want)
			}
			continue
		}
		p, ok := problem.From(err)
		if !ok || p.Type != (problem.Type{Slug: tt.slug}).URI() {
			t.Errorf("%s: got %v, want the problem %s", name, err, tt.slug)
		}
	}
	if got := etag(7); got != `"7"` {
		t.Errorf("got the ETag %s, want \"7\"", got)
	}
}
