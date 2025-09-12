# Mirako CLI

[![Test Status](https://github.com/mirako-ai/mirako-cli/workflows/Test/badge.svg)](https://github.com/mirako-ai/mirako-cli/actions)

The official CLI interface for the [Mirako AI](https://mirako.ai) platform, maintained by the Mirako Team. Create and manage AI avatars, start interactive sessions, generate AI media, and leverage advanced speech services - all from your terminal.

## Features

- **🎭 AI Avatar Management**: Build, generate, list, and manage AI avatars
- **💬 Interactive Sessions**: Launch and manage AI chat sessions
- **🎙️ Speech Services**: Speech-to-text (STT) and text-to-speech (TTS)
- **🎨 Image Generation**: Create AI-generated images from text prompts
- **🎬 Video Creation**: Generate talking avatar videos with custom audio
- **🗣️ Voice Cloning**: Create and manage custom voice profiles
- **🔐 Secure Authentication**: OAuth 2.0 API token management
- **⚡ Fast & Lightweight**: Built in Go for optimal performance

## Installation

### Option 1: Pre-built Binary (Recommended)

Download the latest release for your platform from GitHub Releases.

```bash
# macOS (Apple Silicon)
wget https://github.com/mirako-ai/mirako-cli/releases/latest/download/mirako-darwin-arm64.tar.gz
tar -xzf mirako-darwin-arm64.tar.gz
chmod +x mirako
sudo mv mirako /usr/local/bin/mirako

# Linux
wget https://github.com/mirako-ai/mirako-cli/releases/latest/download/mirako-linux-amd64.tar.gz
tar -xzf mirako-linux-amd64.tar.gz
chmod +x mirako
sudo mv mirako /usr/local/bin/mirako

# Windows
# Download mirako-windows-amd64.exe from releases and add to PATH
```

Afterwards, you can run `mirako` to verify the installation.

### Option 2: Go Install

```bash
go install github.com/mirako-ai/mirako-cli/cmd/mirako@latest

# Ensure GOPATH/bin is in your PATH
export PATH=$PATH:$GOPATH/bin
# Then run
mirako
```

### Option 3: Build from Source

```bash
git clone https://github.com/mirako-ai/mirako-cli.git
cd mirako-cli
go build -o mirako-cli ./cmd/mirako/
sudo mv mirako-cli /usr/local/bin/mirako
```

### Requirements

- **Go 1.24** or later (for building from source)
- **API Token** from mirako.ai

## Quick Start

### 1. Get Your API Token

1. Visit [Mirako Developer Console](https://developer.mirako.ai) and create an account
2. Navigate to [API Keys](https://developer.mirako.ai/api-keys)
3. Generate a new token and copy it

### 2. Configure Authentication

```bash
# Option 1: Interactive setup
mirako auth login

# Option 2: Set via environment variable
export MIRAKO_API_TOKEN="your-api-token-here"

# Option 3: Create config file
mkdir -p ~/.mirako
echo "api_token: your-api-token-here" > ~/.mirako/config.yml
```

### 3. Test Your Setup

```bash
# List your avatars
mirako avatar list

# Generate your first avatar
mirako avatar generate --prompt "A friendly AI assistant with blue eyes"
```


## Configuration

Mirako CLI uses a YAML configuration file located at `~/.mirako/config.yml`. Here's a complete configuration example:

```yaml
# API Configuration
api_url: https://mirako.co
api_token: [my-mirako-api-key]

# Default settings
default_model: metis-2.5        # the model id for Mirako interactive
default_voice: some-voice-id    # default voice profile id used in tts or interactive sessions
default_save_path: .            # Default path to save generated files

# Interactive session profiles
interactive_profiles:
  default:
    avatar_id: [my-avatar-id]
    model: metis-2.5
    llm_model: gemini-2.0-flash
    voice_profile_id: [some-voice-id]
    instruction: "You are a helpful AI assistant."
    tools: ""

# Advanced settings
debug: false
timeout: 30s
```


### Configuration Precedence

1. **CLI flags** (highest priority)
2. **Environment variables**
3. **Configuration file**
4. **Defaults** (lowest priority)

### Environment Variables

```bash
MIRAKO_API_TOKEN    # Your API token
MIRAKO_API_URL      # Custom API URL
MIRAKO_CONFIG       # Custom config file path
MIRAKO_DEBUG        # Enable debug mode
```

## Command Reference

### Global Flags

| Flag | Description | Example |
|------|-------------|---------|
| `--api-token` | API token for authentication | `--api-token abc123` |
| `--api-url` | Custom API URL | `--api-url https://api.mirako.co` |
| `--config` | Custom config file | `--config /path/to/config.yml` |
| `--debug` | Enable debug mode | `--debug` |

### Avatar Commands

```bash
# List all avatars
mirako avatar list

# Generate a new avatar from text prompt
mirako avatar generate --prompt "A professional business woman in a suit"

# Build avatar from image
mirako avatar build --name "My Avatar" --image path/to/photo.jpg

# View avatar details
mirako avatar view [avatar-id]

# Check avatar generation status
mirako avatar status [task-id]

# Delete an avatar
mirako avatar delete [avatar-id]
```

### Interactive Sessions

```bash
# Start a new interactive session using Default profile
mirako interactive start  

# Start a new interactive session using a specific profile
mirako interactive start [interactive-profile-name]

# Start a new interactive session using a specific profile with overrides
mirako interactive start [interactive-profile-name] --instruction "You are an engaging AI assistant that doesn't give up easily."

# Start a new interactive session with custom parameters
mirako interactive start --avatar [avatar-id] --voice [voice-id] --llm-model [model-id] --instruction "You are a helpful assistant"

# List active sessions
mirako interactive list

# Stop sessions
mirako interactive stop [session-id...]
```

### Speech Services

```bash
# Text to speech
mirako speech tts --text "Hello, world!" --voice [voice-id] --output hello.wav

# Speech to text
mirako speech stt --audio path/to/audio.wav --output transcript.txt
```

### Image Generation

```bash
# Generate image from prompt
mirako image generate --prompt "A serene mountain landscape at sunset" --aspect-ratio 16:9
```

### Video Generation

```bash
# Generate talking avatar video
mirako video generate-talking --image path/to/avatar.jpg --audio path/to/audio.wav --output video.mp4
```

### Voice Management

```bash
# List premade voice profiles
mirako voice premade

# List custom voice profiles
mirako voice list

# Get detailed voice profile information
mirako voice view [profile-id]

# Delete a custom voice profile
mirako voice delete [profile-id]
```


### Voice Cloning
```bash

# Voice cloning (Create a custom voice profile)
mirako voice clone --name "My Custom Voice" --annotations path/to/annotation_file --audio-dir path/to/sample_files_dir  

```

> [!NOTE]
> For the best results, ensure your audio samples are high quality and diverse. Using denoising tools on your sample audio files are highly recommended. If you are hesitated on the quality of the voice samples, use the built-in denoiser by passing the `--clean_data` flag.


## Authentication

Mirako CLI uses OAuth 2.0 Bearer token authentication. Your API token is required for all API calls.

### Setting Your Token

```bash
# Method 1: Interactive login
mirako auth login

# Method 2: Environment variable
export MIRAKO_API_TOKEN="your-token-here"

# Method 3: Config file
echo "api_token: your-token-here" > ~/.mirako/config.yml

# Method 4: CLI flag
mirako avatar list --api-token your-token-here
```

## Command Examples

### Complete Avatar Workflow

```bash
# 1. Generate a new avatar
mirako avatar generate --prompt "A friendly AI assistant with glasses and short hair"

# 2. Check generation status (use task ID from step 1)
mirako avatar status [task-id]

# 3. View avatar details (use avatar ID from step 2)
mirako avatar view [avatar-id]

# 4. Use avatar in interactive session
mirako interactive start --avatar [avatar-id]
```

### Image Generation Workflow

```bash
# 1. Generate an image
mirako image generate --prompt "A serene mountain landscape at sunset" --aspect-ratio 16:9

# 2. Check generation status (use task ID from step 1)
mirako image status [task-id]
```

### Video Generation Workflow

```bash
# 1. Generate talking avatar video
mirako video generate-talking --image path/to/avatar.jpg --audio path/to/audio.wav

# 2. Check generation status (use task ID from step 1)
mirako video status [task-id]
```

### Interactive Session Management

```bash
# Start session using Default profile from config.yml
mirako interactive start

# Start session using named profile from config.yml
mirako interactive start CustomerSupport

# Start session with specific avatar (overrides profile)
mirako interactive start --avatar [avatar-id]

# Start session with custom LLM model
mirako interactive start --avatar [avatar-id] --llm-model gemini-2.0-flash

# Start session using profile with flag overrides
mirako interactive start CustomerSupport --voice [different-voice-id]

# Monitor active sessions
mirako interactive list

# Stop multiple sessions
mirako interactive stop [session-id-1] [session-id-2]
```

## Development

### Setup Development Environment

```bash
# Clone the repository
git clone https://github.com/mirako-ai/mirako-cli.git
cd mirako-cli

# Install dependencies
go mod tidy

# Generate API client code
go generate ./internal/api/...

# Build for development
go build -o mirako-cli ./cmd/mirako/

# Run tests
go test ./...
```

### Code Generation

Mirako CLI uses OpenAPI 3.0 specifications to generate API client code:

```bash
# Generate code from OpenAPI spec
go generate ./internal/api/...

# The spec file is located at: spec/openapi-3.0.yaml
# Generated code is in: internal/api/gen_api.go
```

## Troubleshooting

#### Debug Mode

Enable debug mode for detailed logging:

```bash
mirako --debug avatar list
# or
export MIRAKO_DEBUG=true
```

### Getting Help

```bash
# General help
mirako --help

# Command-specific help
mirako avatar --help
mirako avatar generate --help

# Version information
mirako --version
```

## Additional Resources

- **API Documentation**: [docs.mirako.ai](https://docs.mirako.ai)
- **Web Console**: [mirako.ai](https://mirako.ai)

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Acknowledgments

- Built with Cobra CLI framework
- Configuration management with Viper
- API client generation with oapi-codegen
- Interactive prompts with Survey
