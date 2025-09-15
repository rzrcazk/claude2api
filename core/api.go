package core

import (
	"bufio"
	"claude2api/config"
	"claude2api/logger"
	"claude2api/model"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/imroc/req/v3"
)

type Client struct {
	SessionKey   string
	orgID        string
	client       *req.Client
	model        string
	defaultAttrs map[string]interface{}
}

type ResponseEvent struct {
	Type         string `json:"type"`
	Index        int    `json:"index"`
	ContentBlock struct {
		Type string `json:"type"`
	} `json:"content_block"`
	Delta struct {
		Type     string `json:"type"`
		Text     string `json:"text"`
		THINKING string `json:"thinking"`
		// partial_json
		PartialJSON string `json:"partial_json"`
	} `json:"delta"`
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}

func NewClient(sessionKey string, proxy string, model string) *Client {
	client := req.C().ImpersonateChrome().SetTimeout(time.Minute * 5)
	client.Transport.SetResponseHeaderTimeout(time.Second * 10)
	if proxy != "" {
		client.SetProxyURL(proxy)
	}
	// Set common headers
	headers := map[string]string{
		"accept":                    "text/event-stream, text/event-stream",
		"accept-language":           "zh-CN,zh;q=0.9",
		"anthropic-client-platform": "web_claude_ai",
		"content-type":              "application/json",
		"origin":                    config.ConfigInstance.BaseURL,
		"priority":                  "u=1, i",
	}
	
	// 打印客户端初始化信息
	logger.Info(fmt.Sprintf("🔗 [NewClient] 正在初始化Claude API客户端"))
	logger.Info(fmt.Sprintf("🔗 [NewClient] BaseURL: %s", config.ConfigInstance.BaseURL))
	logger.Info(fmt.Sprintf("🔗 [NewClient] Model: %s", model))
	logger.Info(fmt.Sprintf("🔗 [NewClient] SessionKey: %s", sessionKey))
	logger.Info(fmt.Sprintf("🔗 [NewClient] Proxy: %s", proxy))
	
	for key, value := range headers {
		logger.Info(fmt.Sprintf("🔗 [NewClient] 设置通用Header: %s = %s", key, value))
		client.SetCommonHeader(key, value)
	}
	// Set cookies
	client.SetCommonCookies(&http.Cookie{
		Name:  "sessionKey",
		Value: sessionKey,
	})
	// Create default client with session key
	c := &Client{
		SessionKey: sessionKey,
		client:     client,
		model:      model,
		defaultAttrs: map[string]interface{}{
			"personalized_styles": []map[string]interface{}{
				{
					"type":       "default",
					"key":        "Default",
					"name":       "Normal",
					"nameKey":    "normal_style_name",
					"prompt":     "Normal",
					"summary":    "Default responses from Claude",
					"summaryKey": "normal_style_summary",
					"isDefault":  true,
				},
			},
			"tools": []map[string]interface{}{
				{
					"type": "web_search_v0",
					"name": "web_search",
				},
				{"type": "artifacts_v0", "name": "artifacts"},
				{"type": "repl_v0", "name": "repl"},
			},
			"parent_message_uuid": "00000000-0000-4000-8000-000000000000",
			"attachments":         []interface{}{},
			"files":               []interface{}{},
			"sync_sources":        []interface{}{},
			"rendering_mode":      "messages",
			"timezone":            "America/Los_Angeles",
		},
	}
	return c
}

// SetOrgID sets the organization ID for the client
func (c *Client) SetOrgID(orgID string) {
	c.orgID = orgID
}


func (c *Client) GetOrgID() (string, error) {
	url := fmt.Sprintf("%s/api/organizations", config.ConfigInstance.BaseURL)
	
	// 打印详细的请求信息
	logger.Info(fmt.Sprintf("🔗 [GetOrgID] 请求URL: %s", url))
	logger.Info(fmt.Sprintf("🔗 [GetOrgID] 请求方法: GET"))
	logger.Info(fmt.Sprintf("🔗 [GetOrgID] BaseURL: %s", config.ConfigInstance.BaseURL))
	logger.Info(fmt.Sprintf("🔗 [GetOrgID] Referer: %s/new", config.ConfigInstance.BaseURL))
	logger.Info(fmt.Sprintf("🔗 [GetOrgID] SessionKey: %s", c.SessionKey))
	
	resp, err := c.client.R().
		SetHeader("referer", fmt.Sprintf("%s/new", config.ConfigInstance.BaseURL)).
		Get(url)
	if err != nil {
		logger.Error(fmt.Sprintf("🔗 [GetOrgID] 请求失败: %v", err))
		return "", fmt.Errorf("request failed: %w", err)
	}
	
	logger.Info(fmt.Sprintf("🔗 [GetOrgID] 响应状态码: %d", resp.StatusCode))
	logger.Info(fmt.Sprintf("🔗 [GetOrgID] 响应内容: %s", resp.String()))
	
	if resp.StatusCode != http.StatusOK {
		logger.Error(fmt.Sprintf("🔗 [GetOrgID] 意外的状态码: %d", resp.StatusCode))
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	type OrgResponse []struct {
		ID            int    `json:"id"`
		UUID          string `json:"uuid"`
		Name          string `json:"name"`
		RateLimitTier string `json:"rate_limit_tier"`
	}

	var orgs OrgResponse
	if err := json.Unmarshal(resp.Bytes(), &orgs); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}
	if len(orgs) == 0 {
		return "", errors.New("no organizations found")
	}
	if len(orgs) == 1 {
		return orgs[0].UUID, nil
	}
	for _, org := range orgs {
		if org.RateLimitTier == "default_claude_ai" || org.RateLimitTier == "default_claude_max_20x" || org.RateLimitTier == "default_raven_enterprise" {
			return org.UUID, nil
		}
	}
	return "", errors.New("no default organization found")

}

// CreateConversation creates a new conversation and returns its UUID
func (c *Client) CreateConversation() (string, error) {
	if c.orgID == "" {
		return "", errors.New("organization ID not set")
	}
	url := fmt.Sprintf("%s/api/organizations/%s/chat_conversations", config.ConfigInstance.BaseURL, c.orgID)
	
	// 如果以-think结尾
	if strings.HasSuffix(c.model, "-think") {
		c.model = strings.TrimSuffix(c.model, "-think")
		if err := c.UpdateUserSetting("paprika_mode", "extended"); err != nil {
			logger.Error(fmt.Sprintf("Failed to update paprika_mode: %v", err))
		}
	} else {
		if err := c.UpdateUserSetting("paprika_mode", nil); err != nil {
			logger.Error(fmt.Sprintf("Failed to update paprika_mode: %v", err))
		}
	}
	requestBody := map[string]interface{}{
		"model":                            c.model,
		"uuid":                             uuid.New().String(),
		"name":                             "",
		"include_conversation_preferences": true,
	}
	if c.model == "claude-sonnet-4-20250514" {
		// 删除model
		delete(requestBody, "model")
	}

	// 打印详细的请求信息
	requestBodyJSON, _ := json.Marshal(requestBody)
	logger.Info(fmt.Sprintf("🔗 [CreateConversation] 请求URL: %s", url))
	logger.Info(fmt.Sprintf("🔗 [CreateConversation] 请求方法: POST"))
	logger.Info(fmt.Sprintf("🔗 [CreateConversation] BaseURL: %s", config.ConfigInstance.BaseURL))
	logger.Info(fmt.Sprintf("🔗 [CreateConversation] OrgID: %s", c.orgID))
	logger.Info(fmt.Sprintf("🔗 [CreateConversation] Model: %s", c.model))
	logger.Info(fmt.Sprintf("🔗 [CreateConversation] Referer: %s/new", config.ConfigInstance.BaseURL))
	logger.Info(fmt.Sprintf("🔗 [CreateConversation] SessionKey: %s", c.SessionKey))
	logger.Info(fmt.Sprintf("🔗 [CreateConversation] 请求体: %s", string(requestBodyJSON)))

	resp, err := c.client.R().
		SetHeader("referer", fmt.Sprintf("%s/new", config.ConfigInstance.BaseURL)).
		SetBody(requestBody).
		Post(url)
	if err != nil {
		logger.Error(fmt.Sprintf("🔗 [CreateConversation] 请求失败: %v", err))
		return "", fmt.Errorf("request failed: %w", err)
	}
	
	logger.Info(fmt.Sprintf("🔗 [CreateConversation] 响应状态码: %d", resp.StatusCode))
	logger.Info(fmt.Sprintf("🔗 [CreateConversation] 响应内容: %s", resp.String()))
	
	if resp.StatusCode != http.StatusCreated {
		logger.Error(fmt.Sprintf("🔗 [CreateConversation] 意外的状态码: %d", resp.StatusCode))
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	var result map[string]interface{}
	// logger.Info(fmt.Sprintf("create conversation response: %s", resp.String()))
	if err := json.Unmarshal(resp.Bytes(), &result); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}
	logger.Info(fmt.Sprintf("create conversation response: %s", resp.String()))
	uuid, ok := result["uuid"].(string)
	if !ok {
		return "", errors.New("conversation UUID not found in response")
	}
	return uuid, nil
}

// SendMessage sends a message to a conversation and returns the status and response
func (c *Client) SendMessage(conversationID string, message string, stream bool, gc *gin.Context) (int, error) {
	if c.orgID == "" {
		return 500, errors.New("organization ID not set")
	}
	url := fmt.Sprintf("%s/api/organizations/%s/chat_conversations/%s/completion",
		config.ConfigInstance.BaseURL, c.orgID, conversationID)
	
	// Create request body with default attributes
	requestBody := c.defaultAttrs
	requestBody["prompt"] = message
	if c.model != "claude-sonnet-4-20250514" {
		requestBody["model"] = c.model
	}
	
	// 打印详细的请求信息
	requestBodyJSON, _ := json.Marshal(requestBody)
	logger.Info(fmt.Sprintf("🔗 [SendMessage] 请求URL: %s", url))
	logger.Info(fmt.Sprintf("🔗 [SendMessage] 请求方法: POST"))
	logger.Info(fmt.Sprintf("🔗 [SendMessage] BaseURL: %s", config.ConfigInstance.BaseURL))
	logger.Info(fmt.Sprintf("🔗 [SendMessage] OrgID: %s", c.orgID))
	logger.Info(fmt.Sprintf("🔗 [SendMessage] ConversationID: %s", conversationID))
	logger.Info(fmt.Sprintf("🔗 [SendMessage] Model: %s", c.model))
	logger.Info(fmt.Sprintf("🔗 [SendMessage] Stream: %t", stream))
	logger.Info(fmt.Sprintf("🔗 [SendMessage] Referer: %s/chat/%s", config.ConfigInstance.BaseURL, conversationID))
	logger.Info(fmt.Sprintf("🔗 [SendMessage] SessionKey: %s", c.SessionKey))
	logger.Info(fmt.Sprintf("🔗 [SendMessage] Message: %s", message))
	logger.Info(fmt.Sprintf("🔗 [SendMessage] 请求体: %s", string(requestBodyJSON)))
	
	// Set up streaming response
	resp, err := c.client.R().DisableAutoReadResponse().
		SetHeader("referer", fmt.Sprintf("%s/chat/%s", config.ConfigInstance.BaseURL, conversationID)).
		SetHeader("accept", "text/event-stream, text/event-stream").
		SetHeader("anthropic-client-platform", "web_claude_ai").
		SetHeader("cache-control", "no-cache").
		SetBody(requestBody).
		Post(url)
	if err != nil {
		logger.Error(fmt.Sprintf("🔗 [SendMessage] 请求失败: %v", err))
		return 500, fmt.Errorf("request failed: %w", err)
	}
	
	logger.Info(fmt.Sprintf("🔗 [SendMessage] 响应状态码: %d", resp.StatusCode))
	logger.Info(fmt.Sprintf("Claude response status code: %d", resp.StatusCode))
	
	if resp.StatusCode == http.StatusTooManyRequests {
		logger.Error(fmt.Sprintf("🔗 [SendMessage] 速率限制: %d", resp.StatusCode))
		return http.StatusTooManyRequests, fmt.Errorf("rate limit exceeded")
	}
	if resp.StatusCode != http.StatusOK {
		logger.Error(fmt.Sprintf("🔗 [SendMessage] 意外的状态码: %d", resp.StatusCode))
		return resp.StatusCode, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	return 200, c.HandleResponse(resp.Body, stream, gc)
}

// HandleResponse converts Claude's SSE format to OpenAI format and writes to the response writer
func (c *Client) HandleResponse(body io.ReadCloser, stream bool, gc *gin.Context) error {
	defer body.Close()
	// Set headers for streaming
	if stream {
		gc.Writer.Header().Set("Content-Type", "text/event-stream")
		gc.Writer.Header().Set("Cache-Control", "no-cache")
		gc.Writer.Header().Set("Connection", "keep-alive")
		// 发送200状态码
		gc.Writer.WriteHeader(http.StatusOK)
		gc.Writer.Flush()
	}
	scanner := bufio.NewScanner(body)
	clientDone := gc.Request.Context().Done()
	// Keep track of the full response for the final message
	thinkingShown := false
	res_all_text := ""
	partial_json_shown := false
	useTool := false
	useToolEnd := false
	nextLanguage := false
	languageStr := "md"
	for scanner.Scan() {
		select {
		case <-clientDone:
			// 客户端已断开连接，清理资源并退出
			logger.Info("Client closed connection")
			return nil
		default:
			// 继续处理响应
		}
		line := scanner.Text()
		// logger.Info(fmt.Sprintf("Claude SSE line: %s", line))
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := line[6:]
		var event ResponseEvent
		if err := json.Unmarshal([]byte(data), &event); err == nil {
			if event.Type == "error" && event.Error.Message != "" {
				model.ReturnOpenAIResponse(event.Error.Message, stream, gc)
				return nil
			}
			if event.ContentBlock.Type == "tool_use" {
				useTool = true
			}
			if event.ContentBlock.Type == "tool_result" {
				useToolEnd = true
			}
			if event.Type == "content_block_stop" {
				res_text := ""
				if thinkingShown {
					res_text = "</think>\n"
					thinkingShown = false
				}
				if partial_json_shown {
					res_text = "\n```\n"
					partial_json_shown = false
				}
				res_all_text += res_text
				if !stream {
					continue
				}
				model.ReturnOpenAIResponse(res_text, stream, gc)
				continue
			}
			if event.Delta.Type == "text_delta" && event.Delta.Text != "" {
				res_text := event.Delta.Text
				res_all_text += res_text
				if !stream {
					continue
				}
				model.ReturnOpenAIResponse(res_text, stream, gc)
				continue
			}
			if event.Delta.Type == "thinking_delta" {
				res_text := event.Delta.THINKING
				if !thinkingShown {
					res_text = "<think> " + res_text
					thinkingShown = true
				}
				res_all_text += res_text
				if !stream {
					continue
				}
				model.ReturnOpenAIResponse(res_text, stream, gc)
				continue
			}
			if event.Delta.Type == "input_json_delta" {
				res_text := event.Delta.PartialJSON
				//结束使用工具了
				if useTool && res_text == ",\"content\":" {
					useTool = false
					partial_json_shown = false
					continue
				}
				//获取语言,下一次就是了
				if res_text == ",\"language\":" || res_text == ",\"type\":" {
					nextLanguage = true
					continue
				}
				//获取语言注入
				if nextLanguage {
					languageStr = res_text[1:]
					logger.Info(fmt.Sprintf("获取的语言为:%s", languageStr))
					if languageStr == "text/html" {
						languageStr = "html"
					}
					nextLanguage = false
				}
				//使用工具
				if useTool {
					logger.Info(fmt.Sprintf("useTool res_text:%s", res_text))
					continue
				}
				//使用了工具结束拉
				if useToolEnd {
					useToolEnd = false
					continue
				}
				//存在代码首字母为"的情况,特殊处理
				if strings.HasPrefix(res_text, "\"") {
					res_text = res_text[1:]
				}
				//可能会存在多出一个}的情况
				if res_text == "\"}" || res_text == "}" {
					res_text = ""
				}
				//转义
				unquote, err := strconv.Unquote(fmt.Sprintf("\"%s\"", res_text))
				if err == nil {
					res_text = unquote
				} else {
					logger.Error(fmt.Sprintf("转化出错:%s", err.Error()))
					res_text = strings.ReplaceAll(res_text, "\\\\n", "")
					res_text = strings.ReplaceAll(res_text, "\\\\u", "\\u")
					res_text = strings.ReplaceAll(res_text, "\\\"", "\"")
					res_text = strings.ReplaceAll(res_text, "\\\\'", "'")
					res_text = strings.ReplaceAll(res_text, "\\n", "\n")
					res_text = strings.ReplaceAll(res_text, "\\t", "\t")
					res_text = decodeUnicodeEscape(res_text)
				}

				if !partial_json_shown {
					res_text = "\n```" + languageStr + "\n" + res_text
					partial_json_shown = true
				}
				res_all_text += res_text
				if !stream {
					continue
				}
				model.ReturnOpenAIResponse(res_text, stream, gc)
				continue
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading response: %w", err)
	}
	if !stream {
		model.ReturnOpenAIResponse(res_all_text, stream, gc)
	} else {
		// 发送结束标志
		gc.Writer.Write([]byte("data: [DONE]\n\n"))
		gc.Writer.Flush()
	}

	return nil
}
func decodeUnicodeEscape(s string) string {
	var result []rune
	for i := 0; i < len(s); i++ {
		// 检查是否是 Unicode 转义序列
		if len(s)-i >= 6 && s[i:i+2] == "\\u" {
			// 尝试解析 Unicode 码点
			code, err := strconv.ParseInt(s[i+2:i+6], 16, 32)
			if err == nil {
				// 将码点转换为字符
				result = append(result, rune(code))
				// 跳过已处理的 Unicode 转义序列
				i += 5
			} else {
				// 如果解析失败，保留原始字符
				result = append(result, rune(s[i]))
			}
		} else {
			result = append(result, rune(s[i]))
		}
	}
	return string(result)
}

// DeleteConversation deletes a conversation by ID
func (c *Client) DeleteConversation(conversationID string) error {
	if c.orgID == "" {
		return errors.New("organization ID not set")
	}
	url := fmt.Sprintf("%s/api/organizations/%s/chat_conversations/%s",
		config.ConfigInstance.BaseURL, c.orgID, conversationID)
	requestBody := map[string]string{
		"uuid": conversationID,
	}
	
	// 打印详细的请求信息
	requestBodyJSON, _ := json.Marshal(requestBody)
	logger.Info(fmt.Sprintf("🔗 [DeleteConversation] 请求URL: %s", url))
	logger.Info(fmt.Sprintf("🔗 [DeleteConversation] 请求方法: DELETE"))
	logger.Info(fmt.Sprintf("🔗 [DeleteConversation] BaseURL: %s", config.ConfigInstance.BaseURL))
	logger.Info(fmt.Sprintf("🔗 [DeleteConversation] OrgID: %s", c.orgID))
	logger.Info(fmt.Sprintf("🔗 [DeleteConversation] ConversationID: %s", conversationID))
	logger.Info(fmt.Sprintf("🔗 [DeleteConversation] Referer: %s/chat/%s", config.ConfigInstance.BaseURL, conversationID))
	logger.Info(fmt.Sprintf("🔗 [DeleteConversation] SessionKey: %s", c.SessionKey))
	logger.Info(fmt.Sprintf("🔗 [DeleteConversation] 请求体: %s", string(requestBodyJSON)))
	
	resp, err := c.client.R().
		SetHeader("referer", fmt.Sprintf("%s/chat/%s", config.ConfigInstance.BaseURL, conversationID)).
		SetBody(requestBody).
		Delete(url)
	if err != nil {
		logger.Error(fmt.Sprintf("🔗 [DeleteConversation] 请求失败: %v", err))
		return fmt.Errorf("request failed: %w", err)
	}
	
	logger.Info(fmt.Sprintf("🔗 [DeleteConversation] 响应状态码: %d", resp.StatusCode))
	logger.Info(fmt.Sprintf("🔗 [DeleteConversation] 响应内容: %s", resp.String()))
	
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		logger.Error(fmt.Sprintf("🔗 [DeleteConversation] 意外的状态码: %d", resp.StatusCode))
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	return nil
}

// UploadFile uploads files to Claude and adds them to the client's default attributes
// fileData should be in the format: data:image/jpeg;base64,/9j/4AA...
func (c *Client) UploadFile(fileData []string) error {
	if c.orgID == "" {
		return errors.New("organization ID not set")
	}
	if len(fileData) == 0 {
		return errors.New("empty file data")
	}

	// Initialize files array in default attributes if it doesn't exist
	if _, ok := c.defaultAttrs["files"]; !ok {
		c.defaultAttrs["files"] = []interface{}{}
	}

	// Process each file
	for _, fd := range fileData {
		if fd == "" {
			continue // Skip empty entries
		}

		// Parse the base64 data
		parts := strings.SplitN(fd, ",", 2)
		if len(parts) != 2 {
			return errors.New("invalid file data format")
		}

		// Get the content type from the data URI
		metaParts := strings.SplitN(parts[0], ":", 2)
		if len(metaParts) != 2 {
			return errors.New("invalid content type in file data")
		}

		metaInfo := strings.SplitN(metaParts[1], ";", 2)
		if len(metaInfo) != 2 || metaInfo[1] != "base64" {
			return errors.New("invalid encoding in file data")
		}

		contentType := metaInfo[0]

		// Decode the base64 data
		fileBytes, err := base64.StdEncoding.DecodeString(parts[1])
		if err != nil {
			return fmt.Errorf("failed to decode base64 data: %w", err)
		}

		// Determine filename based on content type
		var filename string
		switch contentType {
		case "image/jpeg":
			filename = "image.jpg"
		case "image/png":
			filename = "image.png"
		case "application/pdf":
			filename = "document.pdf"
		default:
			filename = "file"
		}

		// Create the upload URL
		url := fmt.Sprintf("%s/api/%s/upload", config.ConfigInstance.BaseURL, c.orgID)

		// 打印详细的请求信息
		logger.Info(fmt.Sprintf("🔗 [UploadFile] 请求URL: %s", url))
		logger.Info(fmt.Sprintf("🔗 [UploadFile] 请求方法: POST"))
		logger.Info(fmt.Sprintf("🔗 [UploadFile] BaseURL: %s", config.ConfigInstance.BaseURL))
		logger.Info(fmt.Sprintf("🔗 [UploadFile] OrgID: %s", c.orgID))
		logger.Info(fmt.Sprintf("🔗 [UploadFile] Filename: %s", filename))
		logger.Info(fmt.Sprintf("🔗 [UploadFile] ContentType: %s", contentType))
		logger.Info(fmt.Sprintf("🔗 [UploadFile] FileSize: %d bytes", len(fileBytes)))
		logger.Info(fmt.Sprintf("🔗 [UploadFile] Referer: %s/new", config.ConfigInstance.BaseURL))
		logger.Info(fmt.Sprintf("🔗 [UploadFile] SessionKey: %s", c.SessionKey))

		// Create a multipart form request
		resp, err := c.client.R().
			SetHeader("referer", fmt.Sprintf("%s/new", config.ConfigInstance.BaseURL)).
			SetHeader("anthropic-client-platform", "web_claude_ai").
			SetFileBytes("file", filename, fileBytes).
			SetContentType("multipart/form-data").
			Post(url)

		if err != nil {
			logger.Error(fmt.Sprintf("🔗 [UploadFile] 请求失败: %v", err))
			return fmt.Errorf("request failed: %w", err)
		}

		logger.Info(fmt.Sprintf("🔗 [UploadFile] 响应状态码: %d", resp.StatusCode))
		logger.Info(fmt.Sprintf("🔗 [UploadFile] 响应内容: %s", resp.String()))

		if resp.StatusCode != http.StatusOK {
			logger.Error(fmt.Sprintf("🔗 [UploadFile] 意外的状态码: %d", resp.StatusCode))
			return fmt.Errorf("unexpected status code: %d, response: %s", resp.StatusCode, resp.String())
		}

		// Parse the response
		var result struct {
			FileUUID string `json:"file_uuid"`
		}

		if err := json.Unmarshal(resp.Bytes(), &result); err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}

		if result.FileUUID == "" {
			return errors.New("file UUID not found in response")
		}

		// Add file to default attributes
		c.defaultAttrs["files"] = append(c.defaultAttrs["files"].([]interface{}), result.FileUUID)
	}

	return nil
}

func (c *Client) SetBigContext(context string) {
	c.defaultAttrs["attachments"] = []map[string]interface{}{
		{
			"file_name":         "context.txt",
			"file_type":         "text/plain",
			"file_size":         len(context),
			"extracted_content": context,
		},
	}

}

// / UpdateUserSetting updates a single user setting on Claude.ai while preserving all other settings
func (c *Client) UpdateUserSetting(key string, value interface{}) error {
	url := fmt.Sprintf("%s/api/account?statsig_hashing_algorithm=djb2", config.ConfigInstance.BaseURL)
	
	// 打印详细的请求信息
	logger.Info(fmt.Sprintf("🔗 [UpdateUserSetting] 请求URL: %s", url))
	logger.Info(fmt.Sprintf("🔗 [UpdateUserSetting] 请求方法: PUT"))
	logger.Info(fmt.Sprintf("🔗 [UpdateUserSetting] BaseURL: %s", config.ConfigInstance.BaseURL))
	logger.Info(fmt.Sprintf("🔗 [UpdateUserSetting] Setting Key: %s", key))
	logger.Info(fmt.Sprintf("🔗 [UpdateUserSetting] Setting Value: %v", value))
	logger.Info(fmt.Sprintf("🔗 [UpdateUserSetting] SessionKey: %s", c.SessionKey))

	// Default settings structure with all possible fields
	settings := map[string]interface{}{
		"input_menu_pinned_items":          nil,
		"has_seen_mm_examples":             nil,
		"has_seen_starter_prompts":         nil,
		"has_started_claudeai_onboarding":  true,
		"has_finished_claudeai_onboarding": true,
		"dismissed_claudeai_banners":       []interface{}{},
		"dismissed_artifacts_announcement": nil,
		"preview_feature_uses_artifacts":   nil,
		"preview_feature_uses_latex":       nil,
		"preview_feature_uses_citations":   nil,
		"preview_feature_uses_harmony":     nil,
		"enabled_artifacts_attachments":    true,
		"enabled_turmeric":                 nil,
		"enable_chat_suggestions":          nil,
		"dismissed_artifact_feedback_form": nil,
		"enabled_mm_pdfs":                  nil,
		"enabled_gdrive":                   nil,
		"enabled_bananagrams":              nil,
		"enabled_gdrive_indexing":          nil,
		"enabled_web_search":               true,
		"enabled_compass":                  nil,
		"enabled_sourdough":                nil,
		"enabled_foccacia":                 nil,
		"dismissed_claude_code_spotlight":  nil,
		"enabled_geolocation":              nil,
		"enabled_mcp_tools":                nil,
		"paprika_mode":                     nil,
		"enabled_monkeys_in_a_barrel":      nil,
	}

	// Update the specified setting
	if _, exists := settings[key]; exists {
		settings[key] = value
		logger.Info(fmt.Sprintf("Updating setting %s to %v", key, value))
	} else {
		return fmt.Errorf("unknown setting key: %s", key)
	}

	// Create request body
	requestBody := map[string]interface{}{
		"settings": settings,
	}

	// 打印请求体信息
	requestBodyJSON, _ := json.Marshal(requestBody)
	logger.Info(fmt.Sprintf("🔗 [UpdateUserSetting] Referer: %s/new", config.ConfigInstance.BaseURL))
	logger.Info(fmt.Sprintf("🔗 [UpdateUserSetting] Origin: %s", config.ConfigInstance.BaseURL))
	logger.Info(fmt.Sprintf("🔗 [UpdateUserSetting] 请求体: %s", string(requestBodyJSON)))

	// Make the request
	resp, err := c.client.R().
		SetHeader("referer", fmt.Sprintf("%s/new", config.ConfigInstance.BaseURL)).
		SetHeader("origin", config.ConfigInstance.BaseURL).
		SetHeader("anthropic-client-platform", "web_claude_ai").
		SetHeader("cache-control", "no-cache").
		SetHeader("pragma", "no-cache").
		SetHeader("priority", "u=1, i").
		SetBody(requestBody).
		Put(url)

	if err != nil {
		logger.Error(fmt.Sprintf("🔗 [UpdateUserSetting] 请求失败: %v", err))
		return fmt.Errorf("request failed: %w", err)
	}

	logger.Info(fmt.Sprintf("🔗 [UpdateUserSetting] 响应状态码: %d", resp.StatusCode))
	logger.Info(fmt.Sprintf("🔗 [UpdateUserSetting] 响应内容: %s", resp.String()))

	if resp.StatusCode != http.StatusOK && resp.StatusCode != 202 {
		logger.Error(fmt.Sprintf("🔗 [UpdateUserSetting] 意外的状态码: %d", resp.StatusCode))
		return fmt.Errorf("unexpected status code: %d, response: %s", resp.StatusCode, resp.String())
	}

	// logger.Info(fmt.Sprintf("Successfully updated user setting %s: %s", key, resp.String()))
	return nil
}
