package avatar

import "testing"

func TestFormatSupportedInteractiveModels(t *testing.T) {
	tests := []struct {
		name   string
		models *[]string
		want   string
	}{
		{
			name: "nil models",
			want: "-",
		},
		{
			name:   "empty models",
			models: &[]string{},
			want:   "-",
		},
		{
			name:   "populated models",
			models: &[]string{"metis-2.5", "metis-3.0"},
			want:   "metis-2.5, metis-3.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatSupportedInteractiveModels(tt.models); got != tt.want {
				t.Fatalf("formatSupportedInteractiveModels() = %q, want %q", got, tt.want)
			}
		})
	}
}
