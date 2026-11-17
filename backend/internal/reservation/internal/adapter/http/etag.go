package http

import (
	"strconv"
	"strings"

	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/platform/problem"
	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/reservation/internal/app"
)

// etag writes the version of a reservation as a strong entity tag (RFC 9110,
// section 8.8.3): the number in double quotes, such as "3".
func etag(v app.Version) string {
	return `"` + strconv.Itoa(int(v)) + `"`
}

// ifMatch reads the version that a change names in If-Match. Without the
// header the change could overwrite another one unseen, so the operation
// needs it (428). The header names one strong entity tag that this API gave;
// anything else cannot match the current version (412).
func ifMatch(header *string) (app.Version, error) {
	if header == nil {
		return 0, problem.New(problem.PreconditionRequired,
			"Send the ETag of the reservation, as it was read, in If-Match.")
	}
	value := strings.TrimSpace(*header)
	if value == "*" {
		return 0, problem.New(problem.PreconditionRequired,
			"If-Match must name the ETag of the reservation as it was read, not *.")
	}
	digits, ok := strings.CutPrefix(value, `"`)
	if ok {
		digits, ok = strings.CutSuffix(digits, `"`)
	}
	version, err := strconv.Atoi(digits)
	if !ok || err != nil || version < 1 || strconv.Itoa(version) != digits {
		return 0, problem.New(problem.PreconditionFailed,
			"If-Match does not name a version of the reservation. Send the ETag of the reservation unchanged.")
	}
	return app.Version(version), nil
}
