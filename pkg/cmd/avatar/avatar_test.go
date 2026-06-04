package avatar

import (
	"io"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/fatih/color"
	"github.com/mirako-ai/mirako-go/api"
)

func TestPrintAvatarDetailsRestyled(t *testing.T) {
	forceColor(t)

	models := []string{"metis-2.5", "metis-3.0"}
	keyImage := "https://example.test/key.png"
	liveVideo := "https://example.test/live.mp4"
	themes := []api.PresignedAvatarTheme{
		{
			Name:      "default",
			KeyImage:  &keyImage,
			LiveVideo: &liveVideo,
		},
	}
	avatar := api.AvatarResponse{
		Id:                         "avatar-1",
		Name:                       "Avatar One",
		Status:                     api.AvatarResponseStatus("READY"),
		CreatedAt:                  time.Date(2026, 5, 25, 0, 0, 0, 0, time.UTC),
		UserId:                     "user-1",
		SupportedInteractiveModels: &models,
		Themes:                     &themes,
	}

	output := captureStdout(t, func() {
		printAvatarDetails(avatar)
	})

	assertAvatarFieldOnSeparateLines(t, output, "ID", "avatar-1")
	assertAvatarFieldUsesAccentBulletAndBoldLabel(t, output, "ID")
	assertAvatarFieldOnSeparateLines(t, output, "Name", "Avatar One")
	assertAvatarFieldOnSeparateLines(t, output, "Status", "READY")
	assertAvatarFieldOnSeparateLines(t, output, "User ID", "user-1")
	assertAvatarFieldOnSeparateLines(t, output, "Supported Models", "metis-2.5, metis-3.0")
	assertAvatarFieldOnSeparateLines(t, output, "Theme", "default")
	assertAvatarFieldOnSeparateLines(t, output, "Key Image", keyImage)
	assertAvatarFieldOnSeparateLines(t, output, "Live Video", liveVideo)
	if strings.Contains(output, "│") {
		t.Fatalf("output should not contain a guided vertical line: %q", output)
	}
	if strings.Contains(output, "ID: avatar-1") || strings.Contains(output, "Name: Avatar One") {
		t.Fatalf("expected field names and values on separate lines, got %q", output)
	}
}

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

func forceColor(t *testing.T) {
	t.Helper()

	oldNoColor := color.NoColor
	color.NoColor = false
	t.Setenv("NO_COLOR", "")
	t.Cleanup(func() { color.NoColor = oldNoColor })
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create stdout pipe: %v", err)
	}
	defer r.Close()

	os.Stdout = w
	defer func() { os.Stdout = oldStdout }()

	fn()

	if err := w.Close(); err != nil {
		t.Fatalf("failed to close stdout pipe: %v", err)
	}

	output, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("failed to read stdout pipe: %v", err)
	}
	return string(output)
}

func assertAvatarFieldOnSeparateLines(t *testing.T, output, label, value string) {
	t.Helper()
	normalized := stripANSI(output)
	want := "◆ " + label + "\n  " + value
	if !strings.Contains(normalized, want) {
		t.Fatalf("expected output to contain marker field %q and value %q on separate lines; got %q", label, value, output)
	}
}

func assertAvatarFieldUsesAccentBulletAndBoldLabel(t *testing.T, output, label string) {
	t.Helper()
	want := "\x1b[36m◆\x1b[0m \x1b[1m" + label + "\x1b[22m"
	if !strings.Contains(output, want) {
		t.Fatalf("expected output to contain accent marker and bold label %q; got %q", label, output)
	}
}

func stripANSI(output string) string {
	ansiPattern := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	return ansiPattern.ReplaceAllString(output, "")
}
