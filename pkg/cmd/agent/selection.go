package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/mirako-ai/mirako-cli/internal/client"
	promptui "github.com/mirako-ai/mirako-cli/pkg/ui/prompt"
	"github.com/mirako-ai/mirako-go/api"
)

type agentSelectionProvider interface {
	AvatarOptions(ctx context.Context) ([]promptui.SelectOption, error)
	VoiceProfileOptions(ctx context.Context) ([]promptui.SelectOption, error)
}

type apiAgentSelectionProvider struct {
	client *client.Client
}

func (p apiAgentSelectionProvider) AvatarOptions(ctx context.Context) ([]promptui.SelectOption, error) {
	resp, err := p.client.ListAvatars(ctx)
	if err != nil {
		return nil, err
	}
	return avatarSelectOptions(resp), nil
}

func (p apiAgentSelectionProvider) VoiceProfileOptions(ctx context.Context) ([]promptui.SelectOption, error) {
	premadeResp, err := p.client.ListPremadeProfiles(ctx)
	if err != nil {
		return nil, err
	}
	customResp, err := p.client.ListVoiceProfiles(ctx)
	if err != nil {
		return nil, err
	}

	var premadeProfiles *[]api.PresignedVoiceProfile
	if premadeResp != nil {
		premadeProfiles = premadeResp.Data
	}
	var customProfiles *[]api.PresignedVoiceProfile
	if customResp != nil {
		customProfiles = customResp.Data
	}

	options := voiceProfileSelectOptions(premadeProfiles, "premade")
	seen := map[string]struct{}{}
	for _, option := range options {
		seen[option.Result()] = struct{}{}
	}
	for _, option := range voiceProfileSelectOptions(customProfiles, "custom") {
		if _, exists := seen[option.Result()]; exists {
			continue
		}
		options = append(options, option)
		seen[option.Result()] = struct{}{}
	}
	return options, nil
}

func avatarSelectOptions(resp *api.GetUserAvatarListApiResponseBody) []promptui.SelectOption {
	if resp == nil || resp.Data == nil {
		return nil
	}

	options := make([]promptui.SelectOption, 0, len(*resp.Data))
	for _, avatar := range *resp.Data {
		if strings.TrimSpace(avatar.Id) == "" || avatar.Status != api.READY {
			continue
		}
		label := strings.TrimSpace(avatar.Name)
		if label == "" {
			label = avatar.Id
		}

		options = append(options, promptui.SelectOption{
			Label: label,
			Value: avatar.Id,
			Hint:  avatar.Id,
		})
	}
	return options
}

func voiceProfileSelectOptions(profiles *[]api.PresignedVoiceProfile, fallbackKind string) []promptui.SelectOption {
	if profiles == nil {
		return nil
	}

	options := make([]promptui.SelectOption, 0, len(*profiles))
	for _, profile := range *profiles {
		if strings.TrimSpace(profile.Id) == "" {
			continue
		}
		label := strings.TrimSpace(optionalString(profile.Name))
		if label == "" {
			label = profile.Id
		}

		kind := fallbackKind
		if profile.IsPremade != nil {
			if *profile.IsPremade {
				kind = "premade"
			} else {
				kind = "custom"
			}
		}

		descriptionParts := []string{}
		if kind != "" {
			descriptionParts = append(descriptionParts, kind)
		}
		if languages := joinedStrings(profile.Languages); languages != "" {
			descriptionParts = append(descriptionParts, languages)
		}
		if description := strings.TrimSpace(optionalString(profile.Description)); description != "" {
			descriptionParts = append(descriptionParts, description)
		}
		if status := strings.TrimSpace(optionalString(profile.Status)); status != "" {
			descriptionParts = append(descriptionParts, fmt.Sprintf("status: %s", status))
		}

		options = append(options, promptui.SelectOption{
			Label:       label,
			Value:       profile.Id,
			Hint:        profile.Id,
			Description: strings.Join(descriptionParts, " • "),
		})
	}
	return options
}

func joinedStrings(values *[]string) string {
	if values == nil || len(*values) == 0 {
		return ""
	}
	return strings.Join(*values, ", ")
}
