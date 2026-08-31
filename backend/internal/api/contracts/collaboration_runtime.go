package contracts

type CollaborationRuntimeCapabilitiesResponse struct {
	Mode                      string                                 `json:"mode"`
	Ready                     bool                                   `json:"ready"`
	SupportedProtocolVersions []string                               `json:"supportedProtocolVersions"`
	Engines                   []CollaborationRuntimeEngineCapability `json:"engines"`
	SupportedTriggerModes     []string                               `json:"supportedTriggerModes"`
}

type CollaborationRuntimeEngineCapability struct {
	Engine  string `json:"engine"`
	Version string `json:"version"`
	Enabled bool   `json:"enabled"`
	Ready   bool   `json:"ready"`
}
