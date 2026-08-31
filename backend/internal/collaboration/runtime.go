package collaboration

import "context"

const ProtocolVersion = "v1"

// CollaborationRuntime executes one immutable room collaboration snapshot.
// The caller owns persistence and cancels the stream through ctx.
type CollaborationRuntime interface {
	ExecuteConversation(context.Context, Request) (EventStream, error)
}

type CapabilityProvider interface {
	Capabilities(context.Context) (RuntimeCapabilities, error)
}

type RuntimeCapabilities struct {
	Ready                     bool
	SupportedProtocolVersions []string
	Engines                   []EngineCapability
	SupportedTriggerModes     []TriggerMode
}

type EngineCapability struct {
	Engine  Engine
	Version string
	Enabled bool
	Ready   bool
}

func LegacyRuntimeCapabilities() RuntimeCapabilities {
	return RuntimeCapabilities{
		Ready:                     true,
		SupportedProtocolVersions: []string{ProtocolVersion},
		Engines: []EngineCapability{{
			Engine: EngineNative, Version: "legacy-go", Enabled: true, Ready: true,
		}},
		SupportedTriggerModes: []TriggerMode{TriggerMentionOnly},
	}
}

// EventStream yields ordered collaboration events until io.EOF or an error.
type EventStream interface {
	Recv() (Event, error)
}
