package prompt

import (
	"strings"
	"testing"
)

func TestRendererRenderSelectShowsDescriptionsAndSymbols(t *testing.T) {
	renderer := NewRenderer(PlainTheme())
	output := renderer.RenderSelect("Agent type", []SelectOption{
		{
			Label:       "managed agent",
			Value:       "managed_agent",
			Description: "provide prompt, model/tools and host runtime on Mirako",
		},
		{
			Label:       "custom agent",
			Value:       "custom_agent",
			Description: "integrate your existing agent endpoint",
		},
	}, 1)

	for _, want := range []string{
		"◆ Agent type",
		"│   ○ managed agent",
		"│     provide prompt, model/tools and host runtime on Mirako",
		"│ ❯ ● custom agent",
		"│     integrate your existing agent endpoint",
		"│  Use ↑/↓ to choose, Enter to submit",
		"└",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("RenderSelect() missing %q in output:\n%s", want, output)
		}
	}
}

func TestRendererRenderInputAndSubmitted(t *testing.T) {
	renderer := NewRenderer(PlainTheme())
	input := renderer.RenderInput("Interactive model", "metis-2.5")
	for _, want := range []string{"◆ Interactive model", "│  default: metis-2.5", "❯ "} {
		if !strings.Contains(input, want) {
			t.Fatalf("RenderInput() missing %q in output:\n%s", want, input)
		}
	}

	submitted := renderer.RenderSubmitted("Interactive model", "metis-2.5")
	for _, want := range []string{"◇ Interactive model", "│  metis-2.5"} {
		if !strings.Contains(submitted, want) {
			t.Fatalf("RenderSubmitted() missing %q in output:\n%s", want, submitted)
		}
	}
}
