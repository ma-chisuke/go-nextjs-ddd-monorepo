// Package awstest connects the integration tests to AWS Emulator in Docker
// Compose (`make aws-server`). The tests carry the build tag `integration`
// and run in `make check-all`.
package awstest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"

	"github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/platform/awsconfig"
)

// caFile is where tools/scripts/aws.sh puts the CA of the emulator, from the
// root of the repository.
const caFile = ".cache/aws-emulator/ca.pem"

// Config returns the configuration of the AWS SDK for the local emulator. It
// reads the environment that `tools/scripts/aws.sh env` prints, and falls
// back on the local defaults, so that a test also runs from an editor after
// `make aws-server`.
//
// The emulator keeps one state for every test and every checkout, so a test
// names what it creates uniquely (Name).
func Config(t *testing.T) aws.Config {
	t.Helper()
	getenv := func(name string) string {
		if v := os.Getenv(name); v != "" {
			return v
		}
		switch name {
		case awsconfig.RegionVariable:
			return "us-east-1"
		case awsconfig.EndpointVariable:
			return "https://localhost:" + env("AWS_PORT", "19297")
		case awsconfig.CABundleVariable:
			return findCA(t)
		}
		return ""
	}
	// The emulator checks no signature, but the SDK signs every request and
	// needs credentials to do so.
	cfg, err := awsconfig.Load(t.Context(), getenv, config.WithCredentialsProvider(
		credentials.NewStaticCredentialsProvider("emulator", "emulator", "")))
	if err != nil {
		t.Fatalf("awstest: cannot configure the AWS SDK for the local emulator; run `make aws-server` first: %v", err)
	}
	return cfg
}

// findCA looks for the CA of the emulator in the working directory of the
// test and in the directories above it.
func findCA(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		candidate := filepath.Join(dir, filepath.FromSlash(caFile))
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("awstest: no %s above the working directory; run `make aws-server` first", caFile)
		}
		dir = parent
	}
}

func env(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}
