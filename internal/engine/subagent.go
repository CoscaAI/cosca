package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/CoscaAI/cosca/internal/chat"
	"github.com/CoscaAI/cosca/internal/chat/executor"
	"github.com/CoscaAI/cosca/internal/contenttrust"
	"github.com/CoscaAI/cosca/internal/safeerror"
)

// ─── SubagentSpawner ───────────────────────────────────────────────────────────

// SubagentSpawner spawns subagents with isolated context. A subagent receives
// its own system prompt (from the agent definition), a task specification, and
// optional context. It returns a condensed summary of its analysis.
//
// Subagents are intentionally limited:
//   - Spawn makes a single LLM call with no tools (pure analysis).
//   - SpawnWithTools makes an LLM call with limited tool access and one
//     follow-up round if tool calls are invoked.
type SubagentSpawner struct {
	registry *AgentRegistry
	provider ProviderChat
	executor ToolExecutor
}

// NewSubagentSpawner creates a new SubagentSpawner.
func NewSubagentSpawner(registry *AgentRegistry, provider ProviderChat, executor ToolExecutor) *SubagentSpawner {
	return &SubagentSpawner{
		registry: registry,
		provider: provider,
		executor: executor,
	}
}

// SetProvider swaps the LLM provider used by the spawner at runtime.
func (s *SubagentSpawner) SetProvider(p ProviderChat) {
	if p == nil {
		return
	}
	s.provider = p
}

// Spawn spawns a subagent with an isolated LLM call and no tool access.
// The subagent receives:
//   - The agent's system prompt as its identity.
//   - The task as a user message.
//   - Any additional context appended to the task message.
//
// Returns the subagent's summary and token usage.
func (s *SubagentSpawner) Spawn(ctx context.Context, req SubagentRequest) (*SubagentResult, error) {
	agentDef := s.registry.Get(req.Agent)
	if agentDef == nil {
		return &SubagentResult{
			Error:     "The requested subagent was not found.",
			ErrorCode: "subagent_not_found",
		}, nil
	}

	messages := s.buildMessages(agentDef, req)

	opts := chat.ChatOptions{
		Temperature: s.resolveTemperature(agentDef),
		MaxTokens:   req.MaxTokens,
	}

	resp, err := s.provider.Chat(ctx, messages, opts)
	if err != nil {
		return &SubagentResult{
			Error:     safeerror.Message("subagent_call_failed"),
			ErrorCode: "subagent_call_failed",
		}, nil
	}

	content := ""
	tokenUsage := chat.Usage{}
	if len(resp.Choices) > 0 {
		content = resp.Choices[0].Message.Content
	}
	if resp.Usage.TotalTokens > 0 {
		tokenUsage = resp.Usage
	}

	return &SubagentResult{
		Summary:    content,
		TokenUsage: tokenUsage,
	}, nil
}

// SpawnWithTools spawns a subagent with limited tool access. It makes an LLM
// call with the given tools available. If the subagent invokes tools, they are
// executed in a single round and the results are fed back to get the final
// summary. This is intentionally limited to one tool round to prevent runaway
// subagent loops.
func (s *SubagentSpawner) SpawnWithTools(
	ctx context.Context,
	req SubagentRequest,
	tools []chat.Tool,
) (*SubagentResult, error) {
	agentDef := s.registry.Get(req.Agent)
	if agentDef == nil {
		return &SubagentResult{
			Error:     "The requested subagent was not found.",
			ErrorCode: "subagent_not_found",
		}, nil
	}

	messages := s.buildMessages(agentDef, req)

	// Convert chat.Tool interfaces to LLM-compatible tool definitions.
	toolDefs := s.buildToolDefs(tools)

	opts := chat.ChatOptions{
		Temperature: s.resolveTemperature(agentDef),
		MaxTokens:   req.MaxTokens,
		Tools:       toolDefs,
	}

	resp, err := s.provider.Chat(ctx, messages, opts)
	if err != nil {
		return &SubagentResult{
			Error:     safeerror.Message("subagent_call_failed"),
			ErrorCode: "subagent_call_failed",
		}, nil
	}

	totalUsage := chat.Usage{}
	if resp.Usage.TotalTokens > 0 {
		totalUsage = resp.Usage
	}

	content := ""
	toolCalls := []chat.ToolCall{}
	if len(resp.Choices) > 0 {
		content = resp.Choices[0].Message.Content
		toolCalls = resp.Choices[0].Message.ToolCalls
	}

	// If the subagent invoked tools, execute them in a single round and
	// feed the results back for a final summary.
	if len(toolCalls) > 0 {
		// Append assistant message with tool calls.
		messages = append(messages, chat.Message{
			Role:      chat.RoleAssistant,
			Content:   content,
			ToolCalls: toolCalls,
		})

		// Execute each tool call.
		for _, tc := range toolCalls {
			execTC, convErr := s.chatToolCallToExec(tc)
			if convErr != nil {
				messages = append(messages, chat.Message{
					Role:       chat.RoleTool,
					ToolCallID: tc.ID,
					Content:    contenttrust.Envelope(contenttrust.Default(contenttrust.OriginTool, safeerror.ToolMessage(convErr), tc.Function.Name)),
				})
				continue
			}

			result, execErr := s.executor.Execute(ctx, execTC)
			if execErr != nil {
				messages = append(messages, chat.Message{
					Role:       chat.RoleTool,
					ToolCallID: tc.ID,
					Content:    contenttrust.Envelope(contenttrust.Default(contenttrust.OriginTool, safeerror.ToolMessage(execErr), tc.Function.Name)),
				})
				continue
			}

			resultContent := s.formatToolOutput(result)
			messages = append(messages, chat.Message{
				Role:       chat.RoleTool,
				ToolCallID: tc.ID,
				Content:    contenttrust.Envelope(contenttrust.Default(contenttrust.OriginTool, resultContent, tc.Function.Name)),
			})
		}

		// Make a follow-up call without tools to get the final summary.
		followOpts := chat.ChatOptions{
			Temperature: s.resolveTemperature(agentDef),
			MaxTokens:   req.MaxTokens,
		}
		followResp, followErr := s.provider.Chat(ctx, messages, followOpts)
		if followErr != nil {
			return &SubagentResult{
				Summary:    content,
				TokenUsage: totalUsage,
				Error:      safeerror.Message("subagent_followup_failed"),
				ErrorCode:  "subagent_followup_failed",
			}, nil
		}

		if len(followResp.Choices) > 0 {
			content = followResp.Choices[0].Message.Content
		}
		if followResp.Usage.TotalTokens > 0 {
			totalUsage.PromptTokens += followResp.Usage.PromptTokens
			totalUsage.CompletionTokens += followResp.Usage.CompletionTokens
			totalUsage.TotalTokens += followResp.Usage.TotalTokens
		}
	}

	return &SubagentResult{
		Summary:    content,
		TokenUsage: totalUsage,
	}, nil
}

// ─── Internal Helpers ──────────────────────────────────────────────────────────

// buildMessages constructs the message list for a subagent call.
func (s *SubagentSpawner) buildMessages(agentDef *AgentDef, req SubagentRequest) []chat.Message {
	messages := []chat.Message{
		{Role: chat.RoleSystem, Content: agentDef.SystemPrompt},
	}

	taskContent := req.Task
	if len(req.Context) > 0 {
		var b strings.Builder
		b.WriteString(req.Task)
		b.WriteString("\n\n## Context\n\n")
		for k, v := range req.Context {
			b.WriteString(fmt.Sprintf("- **%s**: %s\n", k, v))
		}
		taskContent = b.String()
	}

	messages = append(messages, chat.Message{
		Role:    chat.RoleUser,
		Content: taskContent,
	})
	return messages
}

// buildToolDefs converts chat.Tool interfaces to LLM ToolDefinitions.
func (s *SubagentSpawner) buildToolDefs(tools []chat.Tool) []chat.ToolDefinition {
	defs := make([]chat.ToolDefinition, 0, len(tools))
	for _, t := range tools {
		defs = append(defs, chat.ToolDefinition{
			Type: "function",
			Function: chat.FunctionDef{
				Name:        t.Name(),
				Description: t.Description(),
				Parameters:  s.schemaToMap(t.Schema()),
			},
		})
	}
	return defs
}

// schemaToMap converts a json.RawMessage schema to a map[string]any.
// If unmarshaling fails, it returns a minimal schema map.
func (s *SubagentSpawner) schemaToMap(raw []byte) map[string]any {
	if len(raw) == 0 {
		return map[string]any{"type": "object"}
	}
	var params map[string]any
	if err := json.Unmarshal(raw, &params); err != nil {
		return map[string]any{"type": "object"}
	}
	return params
}

// resolveTemperature returns the effective temperature for a subagent call.
func (s *SubagentSpawner) resolveTemperature(agentDef *AgentDef) float64 {
	if agentDef.Temperature > 0 {
		return agentDef.Temperature
	}
	return 0.7
}

// chatToolCallToExec converts a chat.ToolCall to an executor.ToolCall,
// parsing the JSON arguments string into a map.
func (s *SubagentSpawner) chatToolCallToExec(tc chat.ToolCall) (executor.ToolCall, error) {
	var input map[string]any
	if tc.Function.Arguments != "" {
		if err := json.Unmarshal([]byte(tc.Function.Arguments), &input); err != nil {
			return executor.ToolCall{}, fmt.Errorf("parsing arguments for %q: %w", tc.Function.Name, err)
		}
	}
	return executor.ToolCall{
		ID:    tc.ID,
		Name:  tc.Function.Name,
		Input: input,
	}, nil
}

// formatToolOutput converts an executor.ToolResult to a string for the
// tool result message content.
func (s *SubagentSpawner) formatToolOutput(result *executor.ToolResult) string {
	if result == nil {
		return "No result returned."
	}
	if result.Error != "" {
		return safeerror.ToolMessage(errors.New(result.Error))
	}
	if result.Output == nil {
		return "OK"
	}
	switch v := result.Output.(type) {
	case string:
		return v
	default:
		bytes, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprintf("%v", v)
		}
		return string(bytes)
	}
}

// Ensure time is referenced (for future duration tracking).
var _ = time.Now
