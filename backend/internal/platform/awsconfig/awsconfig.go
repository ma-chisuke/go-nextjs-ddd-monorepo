// Package awsconfig makes the configuration of the AWS SDK for the adapters
// that call AWS (TPL-0013). In AWS the configuration is the region and the
// role of the task; in the local environment it points at AWS Emulator
// (TPL-0017). The adapters do not know which of the two they talk to.
package awsconfig

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/config"

	platformconfig "github.com/ma-chisuke/go-nextjs-ddd-monorepo/backend/internal/platform/config"
)

// The environment variables that Load reads. The first three are the names
// that every AWS SDK and tool uses.
const (
	// RegionVariable holds the AWS region. ECS sets it for every task.
	RegionVariable = "AWS_REGION"
	// EndpointVariable holds the URL of every AWS service, when it is not
	// AWS: the endpoint of AWS Emulator.
	EndpointVariable = "AWS_ENDPOINT_URL"
	// CABundleVariable holds the path of a file with the certificates of the
	// authorities that sign the endpoint, in PEM, next to those of the system:
	// the CA that AWS Emulator makes at every start.
	CABundleVariable = "AWS_CA_BUNDLE"
	// ServerNameVariable holds the name that the certificate of the endpoint
	// carries, when the URL names the endpoint otherwise. AWS Emulator makes
	// its certificate for "localhost", and the other containers of Docker
	// Compose reach it under the name of its service. Only the local
	// environment sets it, and Load refuses it without EndpointVariable.
	ServerNameVariable = "AWS_ENDPOINT_SERVER_NAME"
)

// Load returns the configuration of the AWS SDK from the environment. The
// credentials come from the default chain of the SDK: the role of the task in
// ECS, and the variables AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY locally.
// options change what the SDK loads, as a test does to set its credentials.
//
// The SDK reads its standard variables from the environment of the process
// too. In a service both are the same environment.
func Load(ctx context.Context, getenv func(string) string, options ...func(*config.LoadOptions) error) (aws.Config, error) {
	env := platformconfig.NewReader(getenv)
	region := env.Required(RegionVariable)
	endpoint := env.Default(EndpointVariable, "")
	caBundle := env.Default(CABundleVariable, "")
	serverName := env.Default(ServerNameVariable, "")
	if err := env.Err(); err != nil {
		return aws.Config{}, err
	}
	if serverName != "" && endpoint == "" {
		return aws.Config{}, fmt.Errorf("%s is set without %s: it names the certificate of a local endpoint, and AWS needs none",
			ServerNameVariable, EndpointVariable)
	}

	load := []func(*config.LoadOptions) error{config.WithRegion(region)}
	if endpoint != "" {
		load = append(load, config.WithBaseEndpoint(endpoint))
	}
	if caBundle != "" || serverName != "" {
		client, err := httpClient(caBundle, serverName)
		if err != nil {
			return aws.Config{}, err
		}
		load = append(load, config.WithHTTPClient(client))
	}
	cfg, err := config.LoadDefaultConfig(ctx, append(load, options...)...)
	if err != nil {
		return aws.Config{}, fmt.Errorf("the configuration of the AWS SDK: %w", err)
	}
	return cfg, nil
}

// httpClient returns the HTTP client of the SDK with a TLS configuration that
// trusts the authorities of caBundle too, and expects the certificate of the
// endpoint to carry serverName.
func httpClient(caBundle, serverName string) (*awshttp.BuildableClient, error) {
	var roots *x509.CertPool
	if caBundle != "" {
		pem, err := os.ReadFile(filepath.Clean(caBundle))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", CABundleVariable, err)
		}
		roots, err = x509.SystemCertPool()
		if err != nil {
			roots = x509.NewCertPool()
		}
		if !roots.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("%s: %s holds no certificate in PEM", CABundleVariable, caBundle)
		}
	}
	client := awshttp.NewBuildableClient().WithTransportOptions(func(transport *http.Transport) {
		if transport.TLSClientConfig == nil {
			transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
		}
		if roots != nil {
			transport.TLSClientConfig.RootCAs = roots
		}
		if serverName != "" {
			transport.TLSClientConfig.ServerName = serverName
		}
	})
	return client, nil
}
