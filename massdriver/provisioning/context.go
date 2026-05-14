package provisioning

import "os"

// DeploymentContext is the stable set of identifiers the Massdriver
// platform injects into every provisioner container at deployment time.
// Read via [DeploymentContextFromEnv].
//
// Only fields present in every deployment run live here. Per-phase or
// per-action environment variables (e.g. step-specific paths) are not
// captured — callers that need those should read the env directly.
type DeploymentContext struct {
	BundleName       string
	BundleVersion    string
	DeploymentAction string
	DeploymentID     string
	InstanceID       string
	StepPath         string
	Token            string
}

// DeploymentContextFromEnv reads the standard MASSDRIVER_* deployment
// variables and returns a populated [DeploymentContext]. Missing
// variables come back as empty strings; this function never errors.
//
// Useful from inside a provisioner without first constructing a
// [*Client] — for example, when a bundle's entrypoint needs to know
// which deployment action ("provision" / "decommission" / etc.) it's
// running.
func DeploymentContextFromEnv() DeploymentContext {
	return DeploymentContext{
		BundleName:       os.Getenv("MASSDRIVER_BUNDLE_NAME"),
		BundleVersion:    os.Getenv("MASSDRIVER_BUNDLE_VERSION"),
		DeploymentID:     os.Getenv("MASSDRIVER_DEPLOYMENT_ID"),
		DeploymentAction: os.Getenv("MASSDRIVER_DEPLOYMENT_ACTION"),
		InstanceID:       os.Getenv("MASSDRIVER_INSTANCE_ID"),
		StepPath:         os.Getenv("MASSDRIVER_STEP_PATH"),
		Token:            os.Getenv("MASSDRIVER_TOKEN"),
	}
}
