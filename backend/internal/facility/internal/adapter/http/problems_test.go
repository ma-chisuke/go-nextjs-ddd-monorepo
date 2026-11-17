package http

import (
	"testing"

	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/facility/internal/app"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/platform/problem"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/platform/problem/problemtest"
)

// Every business error has a problem of its own, and every problem has a page.
func TestEveryBusinessErrorHasAProblem(t *testing.T) {
	t.Parallel()
	var kinds []problem.Type
	for _, kind := range app.Kinds() {
		got := problemType(kind)
		if got == problem.InternalError {
			t.Errorf("the business error kind %d has no problem: add it to problemType", kind)
			continue
		}
		kinds = append(kinds, got)
	}
	problemtest.AssertDocumented(t, kinds...)
}
