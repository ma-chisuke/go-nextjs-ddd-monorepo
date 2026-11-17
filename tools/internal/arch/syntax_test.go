package arch

import "testing"

func TestCheckSyntax(t *testing.T) {
	// handler is a file of package pkg that registers a route with call.
	handler := func(pkg, call string) string {
		return "package " + pkg + "\n\nimport \"net/http\"\n\nfunc register(mux *http.ServeMux) {\n\t" + call + "\n}\n"
	}
	runCases(t, []testCase{
		// arch/domain-clock
		{
			name:   "domain-clock fires on time.Now",
			change: put(bookingDomain+"/now.go", "package domain\n\nimport \"time\"\n\nfunc now() time.Time {\n\treturn time.Now()\n}\n"),
			want:   []string{bookingDomain + "/now.go:6: [arch/domain-clock]"},
		},
		{
			name:   "domain-clock fires on a function value",
			change: put(bookingDomain+"/since.go", "package domain\n\nimport \"time\"\n\nvar since = time.Since\n"),
			want:   []string{bookingDomain + "/since.go:5: [arch/domain-clock]"},
		},
		{
			name: "domain-clock fires on an import under another name, and passes the types and constants",
			change: put(bookingDomain+"/wait.go",
				"package domain\n\nimport clock \"time\"\n\nfunc wait(d clock.Duration) {\n\tclock.Sleep(d * clock.Second)\n}\n"),
			want: []string{bookingDomain + "/wait.go:6: [arch/domain-clock]"},
		},
		{
			name: "domain-clock fires on every function that reads or waits for the clock",
			change: put(bookingDomain+"/timers.go", "package domain\n\nimport \"time\"\n\nvar (\n"+
				"\t_ = time.Until\n\t_ = time.After\n\t_ = time.AfterFunc\n\t_ = time.Tick\n"+
				"\t_ = time.NewTimer\n\t_ = time.NewTicker\n)\n"),
			want: []string{
				bookingDomain + "/timers.go:10: [arch/domain-clock]",
				bookingDomain + "/timers.go:11: [arch/domain-clock]",
				bookingDomain + "/timers.go:6: [arch/domain-clock]",
				bookingDomain + "/timers.go:7: [arch/domain-clock]",
				bookingDomain + "/timers.go:8: [arch/domain-clock]",
				bookingDomain + "/timers.go:9: [arch/domain-clock]",
			},
		},
		{
			name:   "domain-clock fires in a package under the domain",
			change: put(bookingDomain+"/period/now.go", "package period\n\nimport \"time\"\n\nvar now = time.Now()\n"),
			want:   []string{bookingDomain + "/period/now.go:5: [arch/domain-clock]"},
		},
		{
			name: "domain-clock passes a name time that is not the package",
			change: put(bookingDomain+"/fake.go",
				"package domain\n\ntype clock struct{}\n\nfunc (clock) Now() int { return 0 }\n\nvar time clock\n\nvar now = time.Now()\n"),
		},
		{
			name:   "domain-clock passes app, which gets the time from a port",
			change: put(bookingApp+"/now.go", "package app\n\nimport \"time\"\n\nvar now = time.Now\n"),
		},

		// arch/route-registration
		{
			name:   "route-registration fires on http.HandleFunc in app",
			change: put(catalogApp+"/routes.go", handler("app", `http.HandleFunc("/rooms", nil)`)),
			want:   []string{catalogApp + "/routes.go:6: [arch/route-registration]"},
		},
		{
			name:   "route-registration fires on ServeMux.Handle in a file of an adapter that is not generated",
			change: put(bookingHTTP+"/extra.go", handler("http", `mux.Handle("/extra", nil)`)),
			want:   []string{bookingHTTP + "/extra.go:6: [arch/route-registration]"},
		},
		{
			name:   "route-registration fires in the root package of a context",
			change: put("backend/internal/catalog/routes.go", handler("catalog", `mux.HandleFunc("/x", nil)`)),
			want:   []string{"backend/internal/catalog/routes.go:6: [arch/route-registration]"},
		},
		{
			name:   "route-registration fires in a command",
			change: put("backend/cmd/catalog/metrics.go", handler("main", `mux.Handle("/metrics", nil)`)),
			want:   []string{"backend/cmd/catalog/metrics.go:6: [arch/route-registration]"},
		},
		{
			name:   "route-registration fires in a platform package other than httpserver",
			change: put(platformClock+"/http.go", handler("clock", `http.Handle("/time", nil)`)),
			want:   []string{platformClock + "/http.go:6: [arch/route-registration]"},
		},
		{
			name:   "route-registration passes a package under httpserver",
			change: put("backend/internal/platform/httpserver/debug/debug.go", handler("debug", `mux.Handle("/debug", nil)`)),
		},
		{
			name:   "route-registration passes other methods of a mux",
			change: put(bookingHTTP+"/serve.go", handler("http", `_, _ = mux.Handler(nil)`)),
		},
	})
}
