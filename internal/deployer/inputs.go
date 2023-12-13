package deployer

import (
	"encoding/json"
	"fmt"
	"os"
)

type Inputs struct {
	ReleaseBucket string                 `json:"releaseBucket"`
	ReleaseKey    string                 `json:"releaseKey"`
	TFStateBucket string                 `json:"tfStateBucket"`
	TFLocksTable  string                 `json:"tfLocksTable"`
	ServiceName   string                 `json:"serviceName"`
	Workspace     string                 `json:"workspace"`
	Region        string                 `json:"region"`
	Operation     string                 `json:"operation"`
	NewState      bool                   `json:"newState"`
	LogURL        string                 `json:"logURL"`
	TFVars        map[string]interface{} `json:"tfVars"`
}

func GetInputs(inputsJSON string) (*Inputs, error) {

	var inputs Inputs
	if err := json.Unmarshal([]byte(os.Args[1]), &inputs); err != nil {
		return nil, fmt.Errorf("error parsing input: %w", err)
	}

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
