package pipeline

import (
	"context"
	"errors"

	"github.com/CoscaAI/cosca/internal/chat"
	"github.com/CoscaAI/cosca/internal/engine"
)

type EngineAdapter struct {
	engine *engine.AgentEngine
}

func NewEngineAdapter(eng *engine.AgentEngine) *EngineAdapter {
	return &EngineAdapter{engine: eng}
}

func (a *EngineAdapter) Engine() *engine.AgentEngine {
	return a.engine
}

func (a *EngineAdapter) Run(ctx context.Context, req RunRequest) (*RunResult, error) {
	history := messagesToChat(req.History)

	eng := a.engine

	if v := req.Options.MaxTurns; v > 0 {
		eng = eng.WithConfigMaxTurns(v)
	}
	if req.Agent != "" {
		eng = eng.WithAgent(req.Agent)
	}

	result, err := eng.Run(ctx, req.Prompt, history)
	if err != nil {
		return nil, err
	}
	if result.Error != "" {
		return nil, errors.New(result.Error)
	}

	return &RunResult{
		Response: result.Content,
		Agent:    req.Agent,
		TokenUsage: TokenUsage{
			Input:  result.TokenUsage.PromptTokens,
			Output: result.TokenUsage.CompletionTokens,
		},
		TurnCount: result.TurnCount,
	}, nil
}

func (a *EngineAdapter) RunStream(ctx context.Context, req RunRequest) (<-chan RunEvent, error) {
	history := messagesToChat(req.History)

	eng := a.engine

	if v := req.Options.MaxTurns; v > 0 {
		eng = eng.WithConfigMaxTurns(v)
	}
	if req.Agent != "" {
		eng = eng.WithAgent(req.Agent)
	}

	srcEvents, err := eng.RunStream(ctx, req.Prompt, history)
	if err != nil {
		return nil, err
	}

	out := make(chan RunEvent, 16)
	go func() {
		defer close(out)
		for ev := range srcEvents {
			out <- convertEngineEvent(ev)
		}
	}()

	return out, nil
}

func convertEngineEvent(ev engine.EngineEvent) RunEvent {
	switch ev.Type {
	case engine.EngineEventContent:
		return RunEvent{Type: EventContent, Data: ev.Content}
	case engine.EngineEventToolStart:
		return RunEvent{Type: EventToolStart, Data: ev.ToolCall}
	case engine.EngineEventToolResult:
		return RunEvent{Type: EventToolResult, Data: ev.ToolResult}
	case engine.EngineEventSubagentStart:
		return RunEvent{Type: EventToolStart, Data: ev.ToolCall}
	case engine.EngineEventSubagentResult:
		return RunEvent{Type: EventToolResult, Data: ev.SubagentResult}
	case engine.EngineEventTurnEnd:
		return RunEvent{Type: EventDone, Data: ev.TurnRecord}
	case engine.EngineEventDone:
		return RunEvent{Type: EventDone}
	case engine.EngineEventError:
		return RunEvent{Type: EventError, Data: ev.Error}
	default:
		return RunEvent{Type: EventContent, Data: ev.Content}
	}
}

func messagesToChat(msgs []Message) []chat.Message {
	out := make([]chat.Message, len(msgs))
	for i, m := range msgs {
		out[i] = chat.Message{
			Role:    chat.Role(m.Role),
			Content: m.Content,
		}
	}
	return out
}
