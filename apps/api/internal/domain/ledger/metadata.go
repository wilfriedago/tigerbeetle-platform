package ledger

import (
	"fmt"
	"regexp"
)

// Metadata limits. Metadata lives in Postgres next to the entity it describes (ADR-0002) and is
// for the client's own systems, so the limits only keep rows and responses small.
const (
	MaxMetadataEntries     = 64
	MaxMetadataKeyLength   = 64
	MaxMetadataValueLength = 512
)

var metadataKeyPattern = regexp.MustCompile(`^[A-Za-z0-9_.:-]{1,64}$`)

// ValidateMetadata checks key names, value lengths and the number of entries.
func ValidateMetadata(metadata map[string]string) error {
	if len(metadata) > MaxMetadataEntries {
		return fmt.Errorf("%w: metadata has at most %d entries, got %d", ErrInvalid, MaxMetadataEntries, len(metadata))
	}
	for key, value := range metadata {
		if !metadataKeyPattern.MatchString(key) {
			return fmt.Errorf("%w: metadata key %q must be 1 to %d letters, digits or _ . : -", ErrInvalid, key, MaxMetadataKeyLength)
		}
		if len(value) > MaxMetadataValueLength {
			return fmt.Errorf("%w: metadata value of %q is longer than %d characters", ErrInvalid, key, MaxMetadataValueLength)
		}
	}
	return nil
}
