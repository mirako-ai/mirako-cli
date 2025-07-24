package speech

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mirako-ai/mirako-cli/internal/api"
	"github.com/mirako-ai/mirako-cli/internal/client"
	"github.com/mirako-ai/mirako-cli/internal/config"
	"github.com/mirako-ai/mirako-cli/internal/errors"
	"github.com/spf13/cobra"
)

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

func NewSpeechCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "speech",
		Short: "Manage speech services",
		Long:  `Convert speech to text (STT) and text to speech (TTS)`,
	}

	cmd.AddCommand(newSTTCmd())
	cmd.AddCommand(newTTSCmd())

	return cmd
}

func newSTTCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stt",
		Short: "Speech to text",
		Long:  `Convert audio to text using speech recognition`,
		RunE:  runSTT,
	}

	cmd.Flags().StringP("audio", "a", "", "Path to the audio file to convert to text")
	cmd.Flags().StringP("output", "o", "", "Output file path for the text (optional, prints to stdout if not provided)")

	return cmd
}

func runSTT(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	audioPath, _ := cmd.Flags().GetString("audio")
	if audioPath == "" {
		return fmt.Errorf("audio file path is required. Use --audio flag")
	}

	outputPath, _ := cmd.Flags().GetString("output")

	// Read and encode the audio file
	audioData, err := os.ReadFile(audioPath)
	if err != nil {
		return fmt.Errorf("failed to read audio file: %w", err)
	}

	// Encode as base64
	encodedAudio := base64.StdEncoding.EncodeToString(audioData)

	client, err := client.New(cfg)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	// Start STT processing with spinner
	fmt.Printf("🎤 Converting speech to text...\n")

	// Show loading spinner
	spinnerTicker := time.NewTicker(100 * time.Millisecond)
	defer spinnerTicker.Stop()

	spinnerIndex := 0
	clearLine := "\r\033[K"

	// Create a channel to receive the result
	resultChan := make(chan *api.STTApiResponseBody, 1)
	errorChan := make(chan error, 1)

	// Start the API call in a goroutine
	go func() {
		resp, err := client.SpeechToText(ctx, encodedAudio)
		if err != nil {
			errorChan <- err
			return
		}
		resultChan <- resp
	}()

	// Wait for result with spinner
	for {
		select {
		case <-ctx.Done():
			fmt.Print(clearLine)
			return fmt.Errorf("operation cancelled: %w", ctx.Err())
		case err := <-errorChan:
			fmt.Print(clearLine)
			if apiErr, ok := errors.IsAPIError(err); ok {
				return fmt.Errorf(apiErr.GetUserFriendlyMessage())
			}
			return fmt.Errorf("failed to convert speech to text: %w", err)
		case resp := <-resultChan:
			fmt.Print(clearLine)
			if resp.Data == nil {
				return fmt.Errorf("unexpected response from server")
			}

			text := resp.Data.Text

			if outputPath != "" {
				// Save to file
				dir := filepath.Dir(outputPath)
				if err := os.MkdirAll(dir, 0755); err != nil {
					return fmt.Errorf("failed to create directory: %w", err)
				}

				if err := os.WriteFile(outputPath, []byte(text), 0644); err != nil {
					return fmt.Errorf("failed to save text: %w", err)
				}

				fmt.Printf("✅ Text saved to: %s\n", outputPath)
			} else {
				// Print to stdout
				fmt.Printf("📝 Transcribed text:\n%s\n", text)
			}

			return nil
		case <-spinnerTicker.C:
			frame := spinnerFrames[spinnerIndex%len(spinnerFrames)]
			fmt.Printf("\r\033[K%s Processing...", frame)
			spinnerIndex++
		}
	}
}

func newTTSCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tts",
		Short: "Text to speech",
		Long:  `Convert text to speech audio using a voice profile`,
		RunE:  runTTS,
	}

	cmd.Flags().StringP("text", "t", "", "Text to convert to speech")
	cmd.Flags().StringP("voice", "v", "", "Voice profile ID to use")
	cmd.Flags().StringP("output", "o", "", "Output file path for the audio file (e.g., ./output/audio.wav)")
	cmd.Flags().StringP("chinese", "c", "", "Chinese language variant (mandarin or yue)")
	cmd.Flags().Float32P("temperature", "T", 1.0, "Temperature for TTS generation (0.0-1.0)")
	cmd.Flags().Float32P("fragment-interval", "f", 0.1, "Fragment interval between sentences (0.0-1.0)")

	return cmd
}

func runTTS(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	text, _ := cmd.Flags().GetString("text")
	if text == "" {
		return fmt.Errorf("text is required. Use --text flag")
	}

	voiceProfileID, _ := cmd.Flags().GetString("voice")
	if voiceProfileID == "" {
		return fmt.Errorf("voice profile ID is required. Use --voice flag")
	}

	outputPath, _ := cmd.Flags().GetString("output")
	chinese, _ := cmd.Flags().GetString("chinese")
	temperature, _ := cmd.Flags().GetFloat32("temperature")
	fragmentInterval, _ := cmd.Flags().GetFloat32("fragment-interval")

	// Prepare TTS parameters
	var chineseLanguage *api.TTSApiRequestBodyChineseLanguage
	if chinese != "" {
		if chinese == "mandarin" {
			chinese := api.Mandarin
			chineseLanguage = &chinese
		} else if chinese == "yue" {
			chinese := api.Yue
			chineseLanguage = &chinese
		} else {
			return fmt.Errorf("invalid chinese language variant. Use 'mandarin' or 'yue'")
		}
	}

	var opts *api.TTSParams
	if temperature != 1.0 || fragmentInterval != 0.1 {
		opts = &api.TTSParams{
			Temperature:      &temperature,
			FragmentInterval: &fragmentInterval,
		}
	}

	client, err := client.New(cfg)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	// Start TTS processing with spinner
	fmt.Printf("🗣️  Converting text to speech...\n")

	// Show loading spinner
	spinnerTicker := time.NewTicker(100 * time.Millisecond)
	defer spinnerTicker.Stop()

	spinnerIndex := 0
	clearLine := "\r\033[K"

	// Create a channel to receive the result
	resultChan := make(chan *api.TTSApiResponseBody, 1)
	errorChan := make(chan error, 1)

	// Start the API call in a goroutine
	go func() {
		resp, err := client.TextToSpeech(ctx, text, voiceProfileID, "b64_audio_str", chineseLanguage, opts)
		if err != nil {
			errorChan <- err
			return
		}
		resultChan <- resp
	}()

	// Wait for result with spinner
	for {
		select {
		case <-ctx.Done():
			fmt.Print(clearLine)
			return fmt.Errorf("operation cancelled: %w", ctx.Err())
		case err := <-errorChan:
			fmt.Print(clearLine)
			if apiErr, ok := errors.IsAPIError(err); ok {
				return fmt.Errorf(apiErr.GetUserFriendlyMessage())
			}
			return fmt.Errorf("failed to convert text to speech: %w", err)
		case resp := <-resultChan:
			fmt.Print(clearLine)
			if resp.Data == nil {
				return fmt.Errorf("unexpected response from server")
			}

			audioData := resp.Data.B64AudioStr
			if audioData == nil {
				return fmt.Errorf("no audio data received from server")
			}

			// Determine output path
			if outputPath == "" {
				defaultFilename := fmt.Sprintf("speech_%s.wav", time.Now().Format("20060102_150405"))
				outputPath = filepath.Join(cfg.DefaultSavePath, defaultFilename)
			}

			// Ensure .wav extension
			if !strings.HasSuffix(strings.ToLower(outputPath), ".wav") {
				outputPath += ".wav"
			}

			// Decode base64 audio
			decodedAudio, err := base64.StdEncoding.DecodeString(*audioData)
			if err != nil {
				return fmt.Errorf("failed to decode audio data: %w", err)
			}

			// Create directory if it doesn't exist
			dir := filepath.Dir(outputPath)
			if err := os.MkdirAll(dir, 0755); err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}

			// Save the audio file
			if err := os.WriteFile(outputPath, decodedAudio, 0644); err != nil {
				return fmt.Errorf("failed to save audio: %w", err)
			}

			fmt.Printf("✅ Audio saved to: %s\n", outputPath)
			if resp.Data.OutputDuration != nil {
				fmt.Printf("📊 Duration: %.2f seconds\n", *resp.Data.OutputDuration)
			}

			return nil
		case <-spinnerTicker.C:
			frame := spinnerFrames[spinnerIndex%len(spinnerFrames)]
			fmt.Printf("\r\033[K%s Generating audio...", frame)
			spinnerIndex++
		}
	}
}

