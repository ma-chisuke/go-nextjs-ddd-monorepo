package arch

import (
	"strings"
	"testing"
)

func TestLocate(t *testing.T) {
	tests := []struct {
		rel  string
		want location
	}{
		{rel: "", want: location{area: areaOutside}},
		{rel: "cmd", want: location{area: areaOutside}},
		{rel: "report", want: location{area: areaOutside}},
		{rel: "cmd/facility", want: location{area: areaCmd, context: "facility"}},
		{rel: "cmd/facility/worker", want: location{area: areaCmd, context: "facility"}},
		{rel: "internal/platform", want: location{area: areaPlatform}},
		{rel: "internal/platform/httpserver", want: location{area: areaPlatform}},
		{rel: "internal/facility", want: location{area: areaContext, context: "facility", layer: layerRoot}},
		{rel: "internal/facility/export", want: location{area: areaContext, context: "facility"}},
		{rel: "internal/facility/internal", want: location{area: areaContext, context: "facility", protected: true}},
		{rel: "internal/facility/internal/shared", want: location{area: areaContext, context: "facility", protected: true}},
		{rel: "internal/facility/internal/domain", want: location{area: areaContext, context: "facility", layer: layerDomain, protected: true}},
		{rel: "internal/facility/internal/domain/period", want: location{area: areaContext, context: "facility", layer: layerDomain, protected: true}},
		{rel: "internal/facility/internal/app", want: location{area: areaContext, context: "facility", layer: layerApp, protected: true}},
		{rel: "internal/facility/internal/adapter", want: location{area: areaContext, context: "facility", protected: true}},
		{
			rel:  "internal/facility/internal/adapter/postgres/postgrestest",
			want: location{area: areaContext, context: "facility", layer: layerAdapter, adapter: "postgres", protected: true},
		},
	}
	for _, tt := range tests {
		if got := locate(tt.rel); got != tt.want {
			t.Errorf("locate(%q) = %+v, want %+v", tt.rel, got, tt.want)
		}
	}
}

func TestCheckWithoutBackend(t *testing.T) {
	findings, err := Check(t.Context(), t.TempDir())
	if err != nil || len(findings) != 0 {
		t.Errorf("Check() = %v, %v; want no findings", findings, err)
	}
}

func TestCheckFileWithSyntaxErrors(t *testing.T) {
	dir := newRepo(t)
	// The parser still reads the imports of a file whose body is broken.
	writeFile(t, dir, bookingDomain+"/broken.go", "package domain\n\nimport \"os\"\n\nfunc broken( {\n")
	want := []string{bookingDomain + "/broken.go:3: [arch/domain-purity]"}
	if got := check(t, dir); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("findings = %q, want %q", got, want)
	}
}

// TestMessages pins the text of one finding of each rule: what an agent reads
// to fix the code.
func TestMessages(t *testing.T) {
	dir := newRepo(t)
	for name, content := range map[string]string{
		bookingApp + "/rooms.go":           goFileImporting("app", imp(catalogApp)),
		"backend/cmd/catalog/booking.go":   goFileImporting("main", imp("backend/internal/booking")),
		platformClock + "/booking.go":      goFileImporting("clock", imp(bookingDomain)),
		bookingHTTP + "/repo.go":           goFileImporting("http", imp(bookingPostgres)),
		bookingDomain + "/clock.go":        goFileImporting("domain", "os"),
		bookingDomain + "/now.go":          "package domain\n\nimport \"time\"\n\nvar now = time.Now()\n",
		bookingPostgres + "/queries/x.sql": "SELECT id FROM catalog.rooms;\n",
		catalogPostgres + "/queries/x.sql": "SELECT id FROM catalog.rooms;\n",
	} {
		writeFile(t, dir, name, content)
	}
	writeFile(t, dir, bookingGlossary, strings.Replace(readFile(t, dir, bookingGlossary), "| `Status` | status | Where a booking stands. |\n", "", 1))
	writeFile(t, dir, bookingJA, strings.Replace(readFile(t, dir, bookingJA), "| `Status` | 状態 | 予約の状態。 |\n", "", 1))
	findings, err := Check(t.Context(), dir)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, f := range findings {
		got[f.Rule] = f.String()
	}
	want := map[string]string{
		RuleContextImport: bookingApp + "/rooms.go:4: [arch/context-import] a package of the context booking imports " +
			catalogApp + ", which belongs to the context catalog. Fix: never import another context: call the HTTP API " +
			"of catalog through a client generated from contracts/http/catalog.openapi.yaml, in an adapter behind a port " +
			"of app, or subscribe to its events. See TPL-0005.",
		RuleModuleImport: "backend/cmd/catalog/booking.go:4: [arch/module-import] this file imports backend/internal/booking, " +
			"the package that wires the context booking together. Fix: only backend/cmd/booking may import it; other code " +
			"reaches booking through its HTTP API or its events. See TPL-0005.",
		RulePlatformImport: platformClock + "/booking.go:4: [arch/platform-import] this platform package imports " +
			bookingDomain + ", which belongs to the context booking. Fix: keep platform free of business code: move this " +
			"code into booking, or let the context hand platform what it needs as a parameter or an interface. See TPL-0004.",
		RuleLayer: bookingHTTP + "/repo.go:4: [arch/layer] the adapter http of the context booking imports " +
			bookingPostgres + ", the adapter postgres. Fix: adapters do not know each other: let each one depend on app " +
			"and domain only, and let module.go wire them together. See TPL-0004.",
		RuleDomainPurity: bookingDomain + "/clock.go:4: [arch/domain-purity] the domain imports os, which is not on the " +
			"allow-list of pure standard library packages. Fix: do the I/O in the use case in app, and pass the result to " +
			"the domain as a value: the current time as a time.Time, a new ID, data that the use case loaded first; " +
			"widening the allow-list in tools/internal/arch needs a decision record (skill write-adr). See TPL-0004.",
		RuleDomainClock: bookingDomain + "/now.go:5: [arch/domain-clock] the domain uses time.Now, which reads the clock. " +
			"Fix: take the current time as a time.Time argument, which the use case gets from its clock port; the package " +
			"time stays for its types, such as time.Time and time.Duration. See TPL-0004.",
		RuleGlossary: bookingGlossary + ":5: [arch/glossary] the exported type `Status` (booking.go:17) is not in the " +
			"glossary. Fix: add a row for `Status` to glossary.md and to its translation: the identifier, the term of the " +
			"business, and what it means. See TPL-0003.",
	}
	for rule, text := range want {
		if got[rule] != text {
			t.Errorf("%s:\n got %s\nwant %s", rule, got[rule], text)
		}
	}
	var sql []string
	for _, f := range findings {
		if f.Rule == RuleSchemaReference {
			sql = append(sql, f.String())
		}
	}
	wantSQL := []string{
		bookingPostgres + "/queries/x.sql:1: [arch/schema-reference] catalog.rooms names the schema of the context " +
			"catalog. Fix: never read, write, or refer to the tables of another context: get the data of catalog through " +
			"its HTTP API or its events, and keep a copy in this context if it must be joined. See TPL-0005.",
		catalogPostgres + "/queries/x.sql:1: [arch/schema-reference] catalog.rooms names the schema of its own context. " +
			"Fix: write rooms without the schema: the search path of the roles of the context picks the schema " +
			"(db/bootstrap.sql), so the SQL stays the same if the context moves. See TPL-0009.",
	}
	if strings.Join(sql, "\n") != strings.Join(wantSQL, "\n") {
		t.Errorf("schema-reference:\n got %s\nwant %s", strings.Join(sql, "\n"), strings.Join(wantSQL, "\n"))
	}
}
