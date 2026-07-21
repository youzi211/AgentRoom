package agentruntimev1

import "fmt"

const ProtocolVersion = "v1"

func ValidateProtocolVersion(version string) error {
	if version != ProtocolVersion {
		return fmt.Errorf("unsupported Agent Runtime protocol version %q", version)
	}
	return nil
}
