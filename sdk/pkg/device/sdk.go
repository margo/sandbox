package device

// wo: workload orchestrator

type DeviceManager struct{}

func NewDeviceManager() {}

func (deviceManager *DeviceManager) GetCapabilities() {}

func (deviceManager *DeviceManager) SubmitCapabilitiesToWO() {}

func (deviceManager *DeviceManager) OnboardWithWO() {}

func (deviceManager *DeviceManager) DeboardWithWO() {}

func (deviceManager *DeviceManager) Reconciliation() {}
