package awsconfig_test

import (
	"encoding/pem"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"

	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/platform/awsconfig"
)

// getenv is the environment of a test. The SDK reads the standard variables
// from the environment of the process as well, where `make test-integration`
// puts those of the local emulator, so the test clears them there.
func getenv(t *testing.T, values map[string]string) func(string) string {
	t.Helper()
	for _, name := range []string{awsconfig.RegionVariable, awsconfig.EndpointVariable, awsconfig.CABundleVariable} {
		t.Setenv(name, "")
	}
	return func(name string) string { return values[name] }
}

// placeholder keeps the SDK from looking for credentials outside the test.
func placeholder() func(*config.LoadOptions) error {
	return config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("test", "test", ""))
}

// endpoint is an HTTPS server like AWS Emulator: its certificate carries the
// names "example.com" and "127.0.0.1", and only caFile vouches for it.
func endpoint(t *testing.T) (server *httptest.Server, caFile string) {
	t.Helper()
	server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)
	caFile = filepath.Join(t.TempDir(), "ca.pem")
	block := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw})
	if err := os.WriteFile(caFile, block, 0o600); err != nil {
		t.Fatal(err)
	}
	return server, caFile
}

// get sends a request with the HTTP client of the configuration.
func get(t *testing.T, cfg aws.Config, url string) error {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	client := cfg.HTTPClient
	if client == nil {
		// What the SDK uses when the configuration names no client.
		client = awshttp.NewBuildableClient()
	}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	return res.Body.Close()
}

func TestLoadReadsTheRegionAndTheEndpoint(t *testing.T) {
	cfg, err := awsconfig.Load(t.Context(), getenv(t, map[string]string{
		awsconfig.RegionVariable:   "ap-northeast-1",
		awsconfig.EndpointVariable: "https://localhost:59297",
	}), placeholder())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Region != "ap-northeast-1" {
		t.Errorf("Region = %q, want ap-northeast-1", cfg.Region)
	}
	if got := aws.ToString(cfg.BaseEndpoint); got != "https://localhost:59297" {
		t.Errorf("BaseEndpoint = %q, want the endpoint of the environment", got)
	}
}

func TestLoadWithoutAnEndpointLeavesAWS(t *testing.T) {
	cfg, err := awsconfig.Load(t.Context(), getenv(t, map[string]string{awsconfig.RegionVariable: "ap-northeast-1"}), placeholder())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.BaseEndpoint != nil {
		t.Errorf("BaseEndpoint = %q, want none", *cfg.BaseEndpoint)
	}
}

func TestLoadTrustsTheCABundle(t *testing.T) {
	server, caFile := endpoint(t)
	env := map[string]string{awsconfig.RegionVariable: "us-east-1", awsconfig.EndpointVariable: server.URL}

	without, err := awsconfig.Load(t.Context(), getenv(t, env), placeholder())
	if err != nil {
		t.Fatal(err)
	}
	if err := get(t, without, server.URL); err == nil {
		t.Fatal("the request passed without the CA: the test proves nothing")
	}

	env[awsconfig.CABundleVariable] = caFile
	with, err := awsconfig.Load(t.Context(), getenv(t, env), placeholder())
	if err != nil {
		t.Fatal(err)
	}
	if err := get(t, with, server.URL); err != nil {
		t.Errorf("the request failed with the CA: %v", err)
	}
}

func TestLoadExpectsTheServerNameInTheCertificate(t *testing.T) {
	server, caFile := endpoint(t)
	// The same server under a name that its certificate does not carry, as
	// another container sees AWS Emulator.
	_, port, err := net.SplitHostPort(server.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	url := "https://localhost:" + port
	env := map[string]string{
		awsconfig.RegionVariable:   "us-east-1",
		awsconfig.EndpointVariable: url,
		awsconfig.CABundleVariable: caFile,
	}

	without, err := awsconfig.Load(t.Context(), getenv(t, env), placeholder())
	if err != nil {
		t.Fatal(err)
	}
	if err := get(t, without, url); err == nil {
		t.Fatal("the request passed under a name that the certificate does not carry: the test proves nothing")
	}

	env[awsconfig.ServerNameVariable] = "example.com"
	with, err := awsconfig.Load(t.Context(), getenv(t, env), placeholder())
	if err != nil {
		t.Fatal(err)
	}
	if err := get(t, with, url); err != nil {
		t.Errorf("the request failed with the server name: %v", err)
	}

	// The name is checked, not skipped: a wrong one fails.
	env[awsconfig.ServerNameVariable] = "another.example"
	wrong, err := awsconfig.Load(t.Context(), getenv(t, env), placeholder())
	if err != nil {
		t.Fatal(err)
	}
	if err := get(t, wrong, url); err == nil {
		t.Error("the request passed with a server name that the certificate does not carry")
	}
}

func TestLoadRefusesABadEnvironment(t *testing.T) {
	notPEM := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(notPEM, []byte("not a certificate"), 0o600); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		env  map[string]string
		want string
	}{
		{"no region", map[string]string{}, awsconfig.RegionVariable},
		{
			"a server name without an endpoint",
			map[string]string{awsconfig.RegionVariable: "us-east-1", awsconfig.ServerNameVariable: "localhost"},
			awsconfig.ServerNameVariable + " is set without " + awsconfig.EndpointVariable,
		},
		{
			"a CA bundle that does not exist",
			map[string]string{awsconfig.RegionVariable: "us-east-1", awsconfig.CABundleVariable: filepath.Join(t.TempDir(), "none.pem")},
			awsconfig.CABundleVariable,
		},
		{
			"a CA bundle without a certificate",
			map[string]string{awsconfig.RegionVariable: "us-east-1", awsconfig.CABundleVariable: notPEM},
			"holds no certificate",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := awsconfig.Load(t.Context(), getenv(t, tt.env), placeholder())
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("Load = %v, want an error that contains %q", err, tt.want)
			}
		})
	}
}
