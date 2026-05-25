package root

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsRootVersionRequest(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected bool
	}{
		{name: "long version", args: []string{"--version"}, expected: true},
		{name: "short version", args: []string{"-v"}, expected: true},
		{name: "no args", args: nil, expected: false},
		{name: "subcommand flag", args: []string{"speech", "tts", "-v", "voice-id"}, expected: false},
		{name: "version with other flag", args: []string{"--debug", "--version"}, expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, isRootVersionRequest(tt.args))
		})
	}
}
