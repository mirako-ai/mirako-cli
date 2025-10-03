# Tools Configuration Example

The `tools` field in the interactive profile now accepts a JSON array instead of a string.

## Example config.yml

```yaml
api_token: your-token-here
api_url: https://mirako.co
default_voice: your-default-voice-id

interactive_profiles:
  default:
    avatar_id: your-avatar-id
    model: metis-2.5
    llm_model: gemini-2.0-flash
    voice_profile_id: your-voice-profile-id
    instruction: You are a helpful AI assistant.
    tools: [{"url":"https://your-tools-domain.com/mcp", "authToken": "some_auth_token"}]
    idle_timeout: 15
  
  advanced:
    avatar_id: your-avatar-id
    model: metis-2.5
    llm_model: gemini-2.0-flash
    voice_profile_id: your-voice-profile-id
    instruction: You are an advanced AI assistant.
    tools:
      - url: https://your-tools-domain.com/mcp
        authToken: some_auth_token
      - url: https://another-tools-domain.com/mcp
        authToken: another_auth_token
    idle_timeout: 30
```

## Notes

- The `tools` field can be an empty array: `tools: []`
- The array will be automatically marshaled to JSON when starting a session
- When using CLI flags, pass the tools as a JSON string: `--tools '[{"url":"some-url"}]'`
