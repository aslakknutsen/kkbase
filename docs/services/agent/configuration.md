# Agent Configuration

Configuration reference for the kkbase Agent service.

## Environment Variables

### MCP Server Connection

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `MCP_SERVER_URL` | MCP server endpoint | `http://kkbase-mcp-server:8080/mcp` | Yes |

### Webhook Configuration

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `WEBHOOK_PORT` | Webhook receiver port | `9090` | No |
| `WEBHOOK_SECRET` | Signature validation secret | - | Yes |

### LLM Configuration

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `LLM_PROVIDER` | LLM service provider | `openai` | Yes |
| `LLM_API_KEY` | LLM API key | - | Yes |
| `LLM_MODEL` | Model name | `gpt-4` | No |
| `LLM_TEMPERATURE` | Response randomness (0-1) | `0.1` | No |

**Supported Providers**:
- `openai` - GPT-4, GPT-3.5-turbo
- `anthropic` - Claude 3, Claude 2
- `gemini` - Gemini Pro

### Logging

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `LOG_LEVEL` | Logging verbosity | `info` | No |

## Example Configurations

### Development

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: kkbase-agent-config
data:
  MCP_SERVER_URL: "http://localhost:8080/mcp"
  WEBHOOK_PORT: "9090"
  LLM_PROVIDER: "openai"
  LLM_MODEL: "gpt-3.5-turbo"  # Cheaper for dev
  LOG_LEVEL: "debug"
```

### Production

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: kkbase-agent-config
data:
  MCP_SERVER_URL: "http://kkbase-mcp-server:8080/mcp"
  WEBHOOK_PORT: "9090"
  LLM_PROVIDER: "openai"
  LLM_MODEL: "gpt-4"
  LLM_TEMPERATURE: "0.1"
  LOG_LEVEL: "info"
```

## See Also

- [Deployment Guide](deployment.md)
- [Integration Guide](integration.md)
- [Agent README](README.md)

