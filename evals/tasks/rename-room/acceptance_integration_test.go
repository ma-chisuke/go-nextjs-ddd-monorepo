//go:build integration

package http_test

// The scenario of acceptance_test.go over the real store: the queries of the
// rename against PostgreSQL, connected as the runtime role of Facility. The
// grader copies both files into backend/internal/facility/internal/adapter/http/
// and runs this test against a database of its own.

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/facility/internal/adapter/postgres"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/platform/postgres/postgrestest"
)

func TestEvalRenameRoomInTheDatabase(t *testing.T) {
	t.Parallel()
	pool := postgrestest.Open(t, "facility")
	prefix := fmt.Sprintf("eval-%d-", time.Now().UnixNano())
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), "DELETE FROM rooms WHERE name LIKE $1", prefix+"%"); err != nil {
			t.Errorf("cannot delete the rooms of the test: %v", err)
		}
	})
	evalRenameScenario(t, evalClientOver(t, postgres.New(pool)), prefix)
}
