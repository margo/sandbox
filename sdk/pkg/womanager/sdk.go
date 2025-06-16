package womanager

// workload orchestration manager
type WOManager struct {
	rootCAPath string
}

func (manager *WOManager) OnboardDevice( /*device info*/ ) {}

func (manager *WOManager) DeboardDevice( /*device id*/ ) {}

func (manager *WOManager) PublishWorkload() {
	// on more than one devices??
}

func (manager *WOManager) GetPublishedWorkloads() {}

func (manager *WOManager) RemovePublishedWorkload() {}

func (manager *WOManager) ReconcilePublishedWorkload() {}

func (manager *WOManager) UpdateDeploymentStatus(deviceID, deploymentID string /*, status interface{}*/) error {
	return nil
}

type RootCAResponse struct {
	Based64Certificate string `json:"certificate"`
}

func (manager *WOManager) GetRootCA() RootCAResponse {
	// the root ca of wo manager
	// the devices use this ca to enable mTLS
	return RootCAResponse{
		Based64Certificate: "",
	}
}
