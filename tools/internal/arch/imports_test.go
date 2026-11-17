package arch

import "testing"

func TestCheckImports(t *testing.T) {
	const (
		bookingRoot    = "backend/internal/booking"
		catalogRoot    = "backend/internal/catalog"
		bookingCatalog = "backend/internal/booking/internal/adapter/catalog"
	)
	runCases(t, []testCase{
		{name: "the fixture passes"},

		// arch/context-import
		{
			name:   "context-import fires on the app layer of another context",
			change: put(bookingApp+"/rooms.go", goFileImporting("app", imp(catalogApp))),
			want:   []string{bookingApp + "/rooms.go:4: [arch/context-import]"},
		},
		{
			name:   "context-import fires in a test file",
			change: put(catalogApp+"/booked_test.go", goFileImporting("app_test", imp(bookingDomain))),
			want:   []string{catalogApp + "/booked_test.go:4: [arch/context-import]"},
		},
		{
			name:   "context-import fires on the root package of another context, and module-import does not",
			change: put(bookingCatalog+"/wiring.go", goFileImporting("catalog", imp(catalogRoot))),
			want:   []string{bookingCatalog + "/wiring.go:4: [arch/context-import]"},
		},
		{
			name:   "context-import fires in the domain, and domain-purity does not",
			change: put(bookingDomain+"/room.go", goFileImporting("domain", imp(catalogApp))),
			want:   []string{bookingDomain + "/room.go:4: [arch/context-import]"},
		},
		{
			name:   "context-import fires in a file that build constraints exclude",
			change: put(catalogApp+"/legacy.go", "//go:build legacy\n\n"+goFileImporting("app", imp(bookingApp))),
			want:   []string{catalogApp + "/legacy.go:6: [arch/context-import]"},
		},
		{
			name: "context-import fires in a package whose every file build constraints exclude",
			change: put("backend/internal/catalog/internal/e2e/e2e_test.go",
				"//go:build e2e\n\n"+goFileImporting("e2e", imp(bookingRoot))),
			want: []string{"backend/internal/catalog/internal/e2e/e2e_test.go:6: [arch/context-import]"},
		},

		// arch/module-import
		{
			name:   "module-import fires on the command of another context",
			change: put("backend/cmd/catalog/booking.go", goFileImporting("main", imp(bookingRoot))),
			want:   []string{"backend/cmd/catalog/booking.go:4: [arch/module-import]"},
		},
		{
			name:   "module-import fires on a package outside cmd and internal",
			change: put("backend/report/report.go", goFileImporting("report", imp(catalogRoot))),
			want:   []string{"backend/report/report.go:4: [arch/module-import]"},
		},
		{
			name: "module-import fires on a package of a context outside its internal directory",
			change: all(
				put(catalogRoot+"/export/export.go", "package export\n"),
				put("backend/report/report.go", goFileImporting("report", imp(catalogRoot+"/export"))),
			),
			want: []string{"backend/report/report.go:4: [arch/module-import]"},
		},
		{
			name:   "module-import passes another command of the context",
			change: put("backend/cmd/booking/worker/worker.go", goFileImporting("worker", imp(bookingRoot))),
		},
		{
			name:   "module-import leaves an import of a package under internal/<context>/internal/ to the compiler",
			change: put("backend/cmd/catalog/app.go", goFileImporting("main", imp(catalogApp))),
		},

		// arch/platform-import
		{
			name:   "platform-import fires on a layer of a context",
			change: put(platformClock+"/booking.go", goFileImporting("clock", imp(bookingDomain))),
			want:   []string{platformClock + "/booking.go:4: [arch/platform-import]"},
		},
		{
			name:   "platform-import fires in a test file, on the root package of a context",
			change: put(platformClock+"/clock_test.go", goFileImporting("clock_test", imp(catalogRoot))),
			want:   []string{platformClock + "/clock_test.go:4: [arch/platform-import]"},
		},

		// arch/layer
		{
			name:   "layer fires when the domain imports app",
			change: put(bookingDomain+"/app.go", goFileImporting("domain", imp(bookingApp))),
			want:   []string{bookingDomain + "/app.go:4: [arch/layer]"},
		},
		{
			name:   "layer fires when the domain imports an adapter",
			change: put(bookingDomain+"/http.go", goFileImporting("domain", imp(bookingHTTP))),
			want:   []string{bookingDomain + "/http.go:4: [arch/layer]"},
		},
		{
			name:   "layer fires when app of a core context imports its adapter/postgres",
			change: put(bookingApp+"/repo.go", goFileImporting("app", imp(bookingPostgres))),
			want:   []string{bookingApp + "/repo.go:4: [arch/layer]"},
		},
		{
			name:   "layer fires when app of a transaction script imports an adapter other than postgres",
			change: put(catalogApp+"/http.go", goFileImporting("app", imp(catalogHTTP))),
			want:   []string{catalogApp + "/http.go:4: [arch/layer]"},
		},
		{
			name: "layer fires when app of a transaction script imports a package under adapter/postgres",
			change: all(
				put(catalogPostgres+"/postgrestest/fake.go", "package postgrestest\n"),
				put(catalogApp+"/fake.go", goFileImporting("app", imp(catalogPostgres+"/postgrestest"))),
			),
			want: []string{catalogApp + "/fake.go:4: [arch/layer]"},
		},
		{
			name:   "layer fires when an adapter imports another adapter",
			change: put(bookingHTTP+"/repo.go", goFileImporting("http", imp(bookingPostgres))),
			want:   []string{bookingHTTP + "/repo.go:4: [arch/layer]"},
		},
		{
			name:   "layer fires when a domain layer makes a transaction script a core context",
			change: put("backend/internal/catalog/internal/domain/room.go", "package domain\n\nfunc capacity() int { return 1 }\n"),
			// The glossary now describes the domain, which exports nothing yet.
			want: []string{
				catalogApp + "/rooms.go:6: [arch/layer]",
				catalogGlossary + ":10: [arch/glossary]",
				catalogGlossary + ":11: [arch/glossary]",
				catalogGlossary + ":12: [arch/glossary]",
				catalogGlossary + ":13: [arch/glossary]",
				catalogGlossary + ":9: [arch/glossary]",
			},
		},
		{
			name:   "layer passes a test that wires layers together",
			change: put(bookingDomain+"/wiring_test.go", goFileImporting("domain_test", imp(bookingApp), imp(bookingPostgres))),
		},

		// arch/domain-purity
		{
			name:   "domain-purity fires on os",
			change: put(bookingDomain+"/clock.go", goFileImporting("domain", "os")),
			want:   []string{bookingDomain + "/clock.go:4: [arch/domain-purity]"},
		},
		{
			name:   "domain-purity fires on context, and passes strings",
			change: put(bookingDomain+"/ctx.go", goFileImporting("domain", "context", "strings")),
			want:   []string{bookingDomain + "/ctx.go:4: [arch/domain-purity]"},
		},
		{
			name: "domain-purity fires on every impure package of the standard library",
			change: put(bookingDomain+"/impure.go", goFileImporting("domain", "database/sql", "net/http", "io",
				"log", "log/slog", "math/rand", "math/rand/v2", "crypto/rand", "sync")),
			want: []string{
				bookingDomain + "/impure.go:10: [arch/domain-purity]",
				bookingDomain + "/impure.go:11: [arch/domain-purity]",
				bookingDomain + "/impure.go:12: [arch/domain-purity]",
				bookingDomain + "/impure.go:4: [arch/domain-purity]",
				bookingDomain + "/impure.go:5: [arch/domain-purity]",
				bookingDomain + "/impure.go:6: [arch/domain-purity]",
				bookingDomain + "/impure.go:7: [arch/domain-purity]",
				bookingDomain + "/impure.go:8: [arch/domain-purity]",
				bookingDomain + "/impure.go:9: [arch/domain-purity]",
			},
		},
		{
			name:   "domain-purity fires on a third-party module",
			change: put(bookingDomain+"/id.go", goFileImporting("domain", "github.com/google/uuid")),
			want:   []string{bookingDomain + "/id.go:4: [arch/domain-purity]"},
		},
		{
			name:   "domain-purity fires on platform",
			change: put(bookingDomain+"/now.go", goFileImporting("domain", imp(platformClock))),
			want:   []string{bookingDomain + "/now.go:4: [arch/domain-purity]"},
		},
		{
			name: "domain-purity fires on a package of its own context outside the domain",
			change: all(
				put("backend/internal/booking/internal/shared/shared.go", "package shared\n"),
				put(bookingDomain+"/shared.go", goFileImporting("domain", imp("backend/internal/booking/internal/shared"))),
			),
			want: []string{bookingDomain + "/shared.go:4: [arch/domain-purity]"},
		},
		{
			name:   "domain-purity fires in a package under the domain",
			change: put(bookingDomain+"/period/io.go", goFileImporting("period", "io")),
			want:   []string{bookingDomain + "/period/io.go:4: [arch/domain-purity]"},
		},
		{
			name: "domain-purity passes the allow-list and the packages of the domain",
			change: put(bookingDomain+"/pure.go", goFileImporting("domain", "cmp", "errors", "fmt", "maps", "math",
				"net/mail", "regexp", "slices", "sort", "strconv", "strings", "time", "unicode", "unicode/utf8",
				imp(bookingDomain+"/period"))),
		},
		{
			name:   "domain-purity passes a test file",
			change: put(bookingDomain+"/io_test.go", goFileImporting("domain", "os", "testing")),
		},
		{
			name:   "domain-purity passes app",
			change: put(bookingApp+"/io.go", goFileImporting("app", "os", "database/sql")),
		},
	})
}
