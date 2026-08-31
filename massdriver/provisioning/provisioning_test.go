package provisioning_test

import (
	"os"
	"testing"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/provisioning"
	"github.com/stretchr/testify/require"
)

var deploymentEnvs = map[string]string{
	"MASSDRIVER_ORGANIZATION_ID":   "ecomm",
	"MASSDRIVER_DEPLOYMENT_ID":     "deploy-123",
	"MASSDRIVER_TOKEN":             "token-abc",
	"MASSDRIVER_BUNDLE_NAME":       "vpc",
	"MASSDRIVER_BUNDLE_VERSION":    "1.0.0",
	"MASSDRIVER_DEPLOYMENT_ACTION": "provision",
	"MASSDRIVER_INSTANCE_ID":       "inst-456",
}

func isolateEnv(t *testing.T) {
	t.Helper()
	for k := range deploymentEnvs {
		t.Setenv(k, "") // register restore-on-cleanup
		os.Unsetenv(k)  // envconfig's bare-name fallback only triggers when truly unset
	}
	for _, k := range []string{"MASSDRIVER_API_KEY", "MASSDRIVER_URL", "MASSDRIVER_PROFILE"} {
		t.Setenv(k, "")
		os.Unsetenv(k)
	}
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", "")
}

func TestNewClient_FromDeploymentEnvs(t *testing.T) {
	isolateEnv(t)
	for k, v := range deploymentEnvs {
		t.Setenv(k, v)
	}

	c, err := provisioning.NewClient()
	require.NoError(t, err)
	cfg := c.Config()
	require.Equal(t, "deploy-123", cfg.DeploymentID)
	require.Equal(t, "token-abc", cfg.Token)
	require.Equal(t, "vpc", cfg.BundleName)
	require.Equal(t, "ecomm", cfg.OrganizationID)
	require.Equal(t, "https://api.massdriver.cloud", cfg.URL)
}

// TestNewClient_IgnoresBareEnvVars: bare TOKEN, DEPLOYMENT_ID, etc.
// must not satisfy the deployment env contract.
func TestNewClient_IgnoresBareEnvVars(t *testing.T) {
	isolateEnv(t)
	t.Setenv("MASSDRIVER_ORGANIZATION_ID", "ecomm")
	for k, v := range deploymentEnvs {
		if k == "MASSDRIVER_ORGANIZATION_ID" {
			continue
		}
		t.Setenv(k[len("MASSDRIVER_"):], v)
	}

	_, err := provisioning.NewClient()
	require.Error(t, err, "bare env vars must not satisfy deployment credential resolution")
}
