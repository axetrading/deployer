package deployer

import (
	"encoding/json"
	"fmt"
	"os"
)

// Inputs represents the input parameters for the deployment process.
type Inputs struct {
	ReleaseBucket string                 `json:"releaseBucket"` // S3 bucket name for storing release artifacts
	ReleaseKey    string                 `json:"releaseKey"`    // S3 key for the release artifact
	TFStateBucket string                 `json:"tfStateBucket"` // S3 bucket name for storing Terraform state
	TFLocksTable  string                 `json:"tfLocksTable"`  // DynamoDB table name for Terraform locks
	ServiceName   string                 `json:"serviceName"`   // Name of the service being deployed
	Workspace     string                 `json:"workspace"`     // Terraform workspace name
	Region        string                 `json:"region"`        // AWS region for deployment
	Operation     string                 `json:"operation"`     // Deployment operation (e.g., "create", "update")
	NewState      bool                   `json:"newState"`      // Flag indicating whether to create a new Terraform state
	LogURL        string                 `json:"logURL"`        // URL for logging the deployment process
	TFVars        map[string]interface{} `json:"tfVars"`        // Additional Terraform variables
}

// GetInputs parses the inputsJSON string and returns the Inputs struct.
func GetInputs(inputsJSON string) (*Inputs, error) {
	var inputs Inputs

	// Unmarshal the inputsJSON string into the inputs struct
	if err := json.Unmarshal([]byte(os.Args[1]), &inputs); err != nil {
		return nil, fmt.Errorf("error parsing input: %w", err)
	}

	// Validate the required input fields
	if inputs.ReleaseBucket == "" {
		return nil, fmt.Errorf("missing releaseBucket")
	}
	if inputs.ReleaseKey == "" {
		return nil, fmt.Errorf("missing releaseKey")
	}
	if inputs.TFStateBucket == "" {
		return nil, fmt.Errorf("missing tfStateBucket")
	}
	if inputs.TFLocksTable == "" {
		return nil, fmt.Errorf("missing tfLocksTable")
	}
	if inputs.ServiceName == "" {
		return nil, fmt.Errorf("missing serviceName")
	}
	if inputs.Workspace == "" {
		return nil, fmt.Errorf("missing workspace")
	}
	if inputs.Region == "" {
		return nil, fmt.Errorf("missing region")
	}

	return &inputs, nil
}
