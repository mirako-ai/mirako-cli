package update

import (
	"errors"
	"testing"

	"github.com/mirako-ai/mirako-cli/internal/updater"
	"github.com/stretchr/testify/assert"
)

func TestFriendlyUpdateError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		contains string
	}{
		{
			name:     "development build",
			err:      updater.ErrDevelopmentBuild,
			contains: "development build",
		},
		{
			name:     "homebrew managed",
			err:      updater.ErrHomebrewManaged,
			contains: "brew upgrade mirako-ai/tap/mirako",
		},
		{
			name:     "other error",
			err:      errors.New("network failed"),
			contains: "network failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := friendlyUpdateError(tt.err)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.contains)
		})
	}
}
