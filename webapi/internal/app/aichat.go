package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"metalx.local/proto/metalxpb"
)

const aiChatSystemPrompt = `你是 MetalX 的自治运维 Agent，运行在 dashboard 的 AIChat 中。

你的职责：
1. 使用工具完整操控 MetalX controller，包括读取集群状态、节点详情、任务、审计、告警、系统设置、dnsmasq/PXE、装机模板、装机任务，以及创建任务和更新 controller 管理的配置。
2. 对用户目标做端到端规划，优先读取状态再执行变更，不要凭空假设节点、模板、MAC、URL 或命令结果。
3. 你可以调用 run_task 在 agent 节点上执行命令；命令必须来自用户明确意图或排障需要，并解释目标节点和风险。
4. 你可以调用 update_dnsmasq_settings、update_runtime_settings、upsert_install_profile、create_install_job 操控 controller；这些是有副作用操作，执行前必须在回复中清楚说明变更内容。如果用户已经明确要求执行，可以直接调用工具。
5. 禁止泄露 API Key、token、密码哈希以外的明文密码；不要把密钥打印到回复。
6. 回复使用中文，给出已执行动作、关键结果、下一步建议。工具失败时说明失败原因和可恢复动作。

可用能力面：
- mxctl/controller: summary、nodes、tasks、audits、alerts、system、runtime settings、dnsmasq/PXE、install profiles/jobs。
- mxagent: 通过 run_task 执行节点命令，通过 dashboard 终端页面进行交互式终端。
- mxapi: 认证、REST/WebSocket 网关、LLM 设置和 AIChat 代理。`

type aiChatMessage struct {
	Role       string          `json:"role"`
	Content    string          `json:"content,omitempty"`
	ToolCalls  []aiToolCall    `json:"tool_calls,omitempty"`
	ToolCallID string          `json:"tool_call_id,omitempty"`
	Name       string          `json:"name,omitempty"`
	Raw        json.RawMessage `json:"-"`
}

type aiToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type aiChatRequest struct {
	Messages   []aiChatMessage `json:"messages"`
	AllowTools bool            `json:"allowTools"`
}

type aiChatCompletionRequest struct {
	Model       string          `json:"model"`
	Messages    []aiChatMessage `json:"messages"`
	Tools       []aiTool        `json:"tools,omitempty"`
	ToolChoice  string          `json:"tool_choice,omitempty"`
	Temperature float64         `json:"temperature"`
}

type aiTool struct {
	Type     string         `json:"type"`
	Function aiToolFunction `json:"function"`
}

type aiToolFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

type aiChatCompletionResponse struct {
	Choices []struct {
		Message aiChatMessage `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (a *App) handleAIChat(c *gin.Context) {
	var payload aiChatRequest
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	settings := a.auth.AISettings()
	if settings.LLMBaseURL == "" || settings.LLMAPIKey == "" || settings.LLMModel == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "LLM endpoint, API key, and model must be configured in system settings"})
		return
	}

	messages := append([]aiChatMessage{{Role: "system", Content: aiChatSystemPrompt}}, payload.Messages...)
	toolResults := make([]map[string]any, 0)
	for i := 0; i < 5; i++ {
		answer, err := a.callOpenAICompatible(settings.LLMBaseURL, settings.LLMAPIKey, settings.LLMModel, settings.LLMTemperature, messages, payload.AllowTools)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
			return
		}
		messages = append(messages, answer)
		if len(answer.ToolCalls) == 0 || !payload.AllowTools {
			c.JSON(http.StatusOK, gin.H{
				"message":     answer,
				"messages":    messages[1:],
				"toolResults": toolResults,
			})
			return
		}
		for _, call := range answer.ToolCalls {
			result := a.executeAITool(c, call.Function.Name, call.Function.Arguments)
			toolResults = append(toolResults, map[string]any{
				"id":     call.ID,
				"name":   call.Function.Name,
				"result": result,
			})
			resultData, _ := json.Marshal(result)
			messages = append(messages, aiChatMessage{
				Role:       "tool",
				ToolCallID: call.ID,
				Name:       call.Function.Name,
				Content:    string(resultData),
			})
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"message":  aiChatMessage{Role: "assistant", Content: "工具调用轮次已达到上限，请缩小目标或分步执行。"},
		"messages": messages[1:],
	})
}

func (a *App) callOpenAICompatible(baseURL, apiKey, model string, temperature float64, messages []aiChatMessage, allowTools bool) (aiChatMessage, error) {
	if temperature == 0 {
		temperature = 0.2
	}
	body := aiChatCompletionRequest{
		Model:       model,
		Messages:    messages,
		Temperature: temperature,
	}
	if allowTools {
		body.Tools = aiTools()
		body.ToolChoice = "auto"
	}
	data, err := json.Marshal(body)
	if err != nil {
		return aiChatMessage{}, err
	}
	endpoint := strings.TrimRight(baseURL, "/") + "/chat/completions"
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(data))
	if err != nil {
		return aiChatMessage{}, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 90 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return aiChatMessage{}, err
	}
	defer resp.Body.Close()
	respData, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode >= 300 {
		return aiChatMessage{}, fmt.Errorf("LLM API returned %s: %s", resp.Status, string(respData))
	}
	var decoded aiChatCompletionResponse
	if err := json.Unmarshal(respData, &decoded); err != nil {
		return aiChatMessage{}, err
	}
	if decoded.Error != nil {
		return aiChatMessage{}, fmt.Errorf("%s", decoded.Error.Message)
	}
	if len(decoded.Choices) == 0 {
		return aiChatMessage{}, fmt.Errorf("LLM API returned no choices")
	}
	return decoded.Choices[0].Message, nil
}

func (a *App) executeAITool(c *gin.Context, name string, rawArgs string) map[string]any {
	var args map[string]any
	if rawArgs != "" && json.Unmarshal([]byte(rawArgs), &args) != nil {
		return map[string]any{"ok": false, "error": "invalid tool arguments JSON"}
	}
	result, err := a.executeAIToolProto(c, name, args)
	if err != nil {
		return map[string]any{"ok": false, "error": err.Error()}
	}
	return map[string]any{"ok": true, "data": result}
}

func (a *App) executeAIToolProto(c *gin.Context, name string, args map[string]any) (any, error) {
	ctx := a.callContext(c)
	switch name {
	case "get_summary":
		return protoMap(a.client.GetSummary(ctx, &metalxpb.Empty{}))
	case "list_nodes":
		return protoMap(a.client.ListNodes(ctx, &metalxpb.Empty{}))
	case "get_node":
		return protoMap(a.client.GetNode(ctx, &metalxpb.NodeID{Id: stringArg(args, "id")}))
	case "list_tasks":
		return protoMap(a.client.ListTasks(ctx, &metalxpb.Empty{}))
	case "list_audits":
		return protoMap(a.client.ListAudits(ctx, &metalxpb.Empty{}))
	case "list_alerts":
		return protoMap(a.client.ListAlerts(ctx, &metalxpb.Empty{}))
	case "get_system":
		return protoMap(a.client.GetSystemInfo(ctx, &metalxpb.Empty{}))
	case "get_runtime_settings":
		return protoMap(a.client.GetAppSettings(ctx, &metalxpb.Empty{}))
	case "update_runtime_settings":
		settings := &metalxpb.AppSettings{}
		_ = mapToProto(args, settings)
		return protoMap(a.client.UpdateAppSettings(ctx, &metalxpb.UpdateAppSettingsRequest{Settings: settings, Actor: "aichat-agent"}))
	case "get_dnsmasq_settings":
		return protoMap(a.client.GetDnsmasqSettings(ctx, &metalxpb.Empty{}))
	case "update_dnsmasq_settings":
		settings := &metalxpb.DnsmasqSettings{}
		_ = mapToProto(args, settings)
		return protoMap(a.client.UpdateDnsmasqSettings(ctx, &metalxpb.UpdateDnsmasqSettingsRequest{Settings: settings, Actor: "aichat-agent"}))
	case "list_install_profiles":
		return protoMap(a.client.ListInstallProfiles(ctx, &metalxpb.Empty{}))
	case "upsert_install_profile":
		profile := &metalxpb.InstallProfile{}
		_ = mapToProto(args, profile)
		return protoMap(a.client.UpsertInstallProfile(ctx, &metalxpb.UpsertInstallProfileRequest{Profile: profile, Actor: "aichat-agent"}))
	case "list_install_jobs":
		return protoMap(a.client.ListInstallJobs(ctx, &metalxpb.Empty{}))
	case "create_install_job":
		return protoMap(a.client.CreateInstallJob(ctx, &metalxpb.CreateInstallJobRequest{
			ProfileId:  stringArg(args, "profileId"),
			MacAddress: stringArg(args, "macAddress"),
			Hostname:   stringArg(args, "hostname"),
			NodeId:     stringArg(args, "nodeId"),
			Actor:      "aichat-agent",
		}))
	case "get_install_job":
		return protoMap(a.client.GetInstallJob(ctx, &metalxpb.InstallJobID{Id: stringArg(args, "id")}))
	case "run_task":
		targets := stringSliceArg(args, "targets")
		return protoMap(a.client.RunTask(ctx, &metalxpb.RunTaskRequest{
			Command: stringArg(args, "command"),
			Targets: targets,
			Actor:   "aichat-agent",
		}))
	default:
		return nil, fmt.Errorf("unknown tool %q", name)
	}
}

func protoMap(message proto.Message, err error) (any, error) {
	if err != nil {
		return nil, err
	}
	data, err := protojson.MarshalOptions{UseProtoNames: false, EmitUnpopulated: true}.Marshal(message)
	if err != nil {
		return nil, err
	}
	var out any
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func mapToProto(value map[string]any, message proto.Message) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return protojson.Unmarshal(data, message)
}

func stringArg(args map[string]any, key string) string {
	if value, ok := args[key].(string); ok {
		return value
	}
	return ""
}

func stringSliceArg(args map[string]any, key string) []string {
	raw, ok := args[key].([]any)
	if !ok {
		return nil
	}
	items := make([]string, 0, len(raw))
	for _, item := range raw {
		if value, ok := item.(string); ok && value != "" {
			items = append(items, value)
		}
	}
	return items
}

func aiTools() []aiTool {
	return []aiTool{
		tool("get_summary", "读取集群总览", object(nil, nil)),
		tool("list_nodes", "列出所有节点", object(nil, nil)),
		tool("get_node", "读取单个节点详情", object(map[string]any{"id": stringSchema("node id")}, []string{"id"})),
		tool("list_tasks", "列出任务历史", object(nil, nil)),
		tool("list_audits", "列出审计记录", object(nil, nil)),
		tool("list_alerts", "列出当前告警", object(nil, nil)),
		tool("get_system", "读取 controller 系统信息", object(nil, nil)),
		tool("get_runtime_settings", "读取 controller 运行配置", object(nil, nil)),
		tool("update_runtime_settings", "更新 controller 运行配置，字段使用 camelCase AppSettings JSON", object(nil, nil)),
		tool("get_dnsmasq_settings", "读取 dnsmasq/PXE 配置和渲染预览", object(nil, nil)),
		tool("update_dnsmasq_settings", "更新 dnsmasq/PXE 配置，字段使用 camelCase DnsmasqSettings JSON", object(nil, nil)),
		tool("list_install_profiles", "列出装机模板", object(nil, nil)),
		tool("upsert_install_profile", "创建或更新装机模板，字段使用 camelCase InstallProfile JSON", object(nil, nil)),
		tool("list_install_jobs", "列出装机任务", object(nil, nil)),
		tool("create_install_job", "创建装机任务", object(map[string]any{
			"profileId":  stringSchema("install profile id"),
			"macAddress": stringSchema("target MAC address"),
			"hostname":   stringSchema("optional hostname"),
			"nodeId":     stringSchema("optional node id"),
		}, []string{"profileId", "macAddress"})),
		tool("get_install_job", "读取装机任务详情", object(map[string]any{"id": stringSchema("install job id")}, []string{"id"})),
		tool("run_task", "在一个或多个 agent 节点执行命令", object(map[string]any{
			"command": stringSchema("shell command"),
			"targets": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
		}, []string{"command", "targets"})),
	}
}

func tool(name, description string, parameters map[string]any) aiTool {
	return aiTool{Type: "function", Function: aiToolFunction{Name: name, Description: description, Parameters: parameters}}
}

func object(properties map[string]any, required []string) map[string]any {
	if properties == nil {
		properties = map[string]any{}
	}
	return map[string]any{"type": "object", "properties": properties, "required": required}
}

func stringSchema(description string) map[string]any {
	return map[string]any{"type": "string", "description": description}
}
