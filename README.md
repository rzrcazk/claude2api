# Claude2Api
Transform Claude's web service into an API service, supporting image recognition, file upload, streaming transmission, thing output... 
The API supports access in the OpenAI format.

[![Go Report Card](https://goreportcard.com/badge/github.com/yushangxiao/claude2api)](https://goreportcard.com/report/github.com/yushangxiao/claude2api)
[![License](https://img.shields.io/github/license/yushangxiao/claude2api)](LICENSE)
|[中文](https://github.com/yushangxiao/claude2api/blob/main/docs/chinses.md)


NOTICE: ONLY PRO USER CAN USE ALL MODELS , FREE USER ONLY CAN USE claude-sonnet-4-20250514

## ✨ Features

- 🖼️ **Image Recognition** - Send images to Claude for analysis
- 📝 **Automatic Conversation Management** -  Conversation can be automatically deleted after use
- 🌊 **Streaming Responses** - Get real-time streaming outputs from Claude
- 📁 **File Upload Support** - Upload long context
- 🧠 **Thinking Process** - Access Claude's step-by-step reasoning, support <think>
- 🔄 **Chat History Management** - Control the length of conversation context , exceeding will upload file
- 🌐 **Proxy Support** - Route requests through your preferred proxy
- 🔐 **API Key Authentication** - Secure your API endpoints
- 🔁 **Automatic Retry** - Feature to automatically retry requests when request fail
- 🌐 **Direct Proxy** -let sk-ant-sid01* as key to use

## 📋 Prerequisites

- Go 1.23+ (for building from source)
- Docker (for containerized deployment)

## 🚀 Deployment Options

### Docker

```bash
docker run -d \
  -p 8080:8080 \
  -e SESSIONS=sk-ant-sid01-xxxx,sk-ant-sid01-yyyy \
  -e APIKEY=123 \
  -e BASE_URL=https://claude.ai \
  -e CHAT_DELETE=true \
  -e MAX_CHAT_HISTORY_LENGTH=10000 \
  -e NO_ROLE_PREFIX=false \
  -e PROMPT_DISABLE_ARTIFACTS=false \
  -e ENABLE_MIRROR_API=false \
  -e MIRROR_API_PREFIX=/mirror \
  --name claude2api \
  ghcr.io/yushangxiao/claude2api:latest
```

### Docker Compose

Create a `docker-compose.yml` file:

```yaml
version: '3'
services:
  claude2api:
    image: ghcr.io/yushangxiao/claude2api:latest
    container_name: claude2api
    ports:
      - "8080:8080"
    environment:
      - SESSIONS=sk-ant-sid01-xxxx,sk-ant-sid01-yyyy
      - ADDRESS=0.0.0.0:8080
      - APIKEY=123
      - PROXY=http://proxy:2080  # Optional
      - BASE_URL=https://claude.ai  # Custom Claude domain
      - CHAT_DELETE=true
      - MAX_CHAT_HISTORY_LENGTH=10000
      - NO_ROLE_PREFIX=false
      - PROMPT_DISABLE_ARTIFACTS=true
      - ENABLE_MIRROR_API=false
      - MIRROR_API_PREFIX=/mirror
    restart: unless-stopped

```

Then run:

```bash
docker-compose up -d
```

### Hugging Face Spaces

You can deploy this project to Hugging Face Spaces with Docker:

1. Fork the Hugging Face Space at [https://huggingface.co/spaces/rclon/claude2api](https://huggingface.co/spaces/rclon/claude2api)
2. Configure your environment variables in the Settings tab
3. The Space will automatically  deploy the Docker image

notice: In Hugging Face, /v1 might be blocked, you can use /hf/v1 instead.
### Direct Deployment

```bash
# Clone the repository
git clone https://github.com/yushangxiao/claude2api.git
cd claude2api

# Create environment file
cat > .env << 'EOF'
SESSIONS=sk-ant-sid01-xxxx,sk-ant-sid01-yyyy
ADDRESS=0.0.0.0:8080
APIKEY=your_api_key_here
PROXY=
BASE_URL=https://claude.ai
CHAT_DELETE=true
MAX_CHAT_HISTORY_LENGTH=10000
NO_ROLE_PREFIX=false
PROMPT_DISABLE_ARTIFACTS=false
ENABLE_MIRROR_API=false
MIRROR_API_PREFIX=
EOF

# Edit the .env file with your actual values
vim .env  

# Build the binary
go build -o claude2api .

./claude2api
```

## ⚙️ Configuration

### YAML Configuration

You can configure Claude2API using a `config.yaml` file in the application's root directory. If this file exists, it will be used instead of environment variables.

Example `config.yaml`:

```yaml
# Sessions configuration
sessions:
  - sessionKey: "sk-ant-sid01-xxxx"
    orgID: ""
  - sessionKey: "sk-ant-sid01-yyyy"
    orgID: ""

# Server address
address: "0.0.0.0:8080"

# API authentication key
apiKey: "123"

# Custom Claude API base URL (replace claude.ai domain)
baseURL: "https://claude.ai"

# Other configuration options...
chatDelete: true
maxChatHistoryLength: 10000
noRolePrefix: false
promptDisableArtifacts: false
enableMirrorApi: false
mirrorApiPrefix: ""
```

A sample configuration file is provided as `config.yaml.example` in the repository.

### Environment Variables

If `config.yaml` doesn't exist, the application will use environment variables for configuration:

| Environment Variable | Description | Default |
|----------------------|-------------|---------|
| `SESSIONS` | Comma-separated list of Claude API session keys | Required |
| `ADDRESS` | Server address and port | `0.0.0.0:8080` |
| `APIKEY` | API key for authentication | Required |
| `PROXY` | HTTP proxy URL | Optional |
| `BASE_URL` | Custom Claude API base URL (replace claude.ai domain) | `https://claude.ai` |
| `CHAT_DELETE` | Whether to delete chat sessions after use | `true` |
| `MAX_CHAT_HISTORY_LENGTH` | Exceeding will text to file | `10000` |
| `NO_ROLE_PREFIX` | Do not add role in every message | `false` |
| `PROMPT_DISABLE_ARTIFACTS` | Add Prompt try to disable Artifacts | `false` |
| `ENABLE_MIRROR_API` | Enable direct use sk-ant-* as key | `false` |
| `MIRROR_API_PREFIX` | Add Prefix to protect Mirror，required when ENABLE_MIRROR_API is true | `` |

## 🌐 Custom Domain Usage

### Problem with claude.ai Access

If you cannot access `claude.ai` directly due to network restrictions or DNS issues, you can now use a custom domain that proxies to Claude's API.

### Solution: Custom Base URL

Set the `BASE_URL` environment variable or `baseURL` in config.yaml to point to your custom domain:

**Environment Variable:**
```bash
BASE_URL=https://your-custom-claude-domain.com
```

**YAML Configuration:**
```yaml
baseURL: "https://your-custom-claude-domain.com"
```

**Docker Example:**
```bash
docker run -d \
  -p 8080:8080 \
  -e SESSIONS=sk-ant-sid01-xxxx \
  -e APIKEY=123 \
  -e BASE_URL=https://your-custom-claude-domain.com \
  --name claude2api \
  ghcr.io/yushangxiao/claude2api:latest
```

### Requirements for Custom Domain

Your custom domain should:
1. Proxy all requests to `https://claude.ai`
2. Maintain the same API paths and structure
3. Forward all headers correctly
4. Support HTTPS

This eliminates the need for proxy configuration while providing the same functionality.

## 📝 API Usage

### Authentication

Include your API key in the request header:

```
Authorization: Bearer YOUR_API_KEY
```

### Chat Completion

```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -d '{
    "model": "claude-3-7-sonnet-20250219",
    "messages": [
      {
        "role": "user",
        "content": "Hello, Claude!"
      }
    ],
    "stream": true
  }'
```

### Image Analysis

```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -d '{
    "model": "claude-3-7-sonnet-20250219",
    "messages": [
      {
        "role": "user",
        "content": [
          {
            "type": "text",
            "text": "What\'s in this image?"
          },
          {
            "type": "image_url",
            "image_url": {
              "url": "data:image/jpeg;base64,..."
            }
          }
        ]
      }
    ]
  }'
```


## 🤝 Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- [Anthropic](https://www.anthropic.com/) for creating Claude
- The Go community for the amazing ecosystem

---
 ## 🎁 Support

If you find this project helpful, consider supporting me on [Afdian](https://afdian.com/a/iscoker)  😘

## Star History

[![Star History Chart](https://api.star-history.com/svg?repos=yushangxiao/claude2api&type=Date)](https://www.star-history.com/#yushangxiao/claude2api&Date)

Made with ❤️ by [yushangxiao](https://github.com/yushangxiao)
