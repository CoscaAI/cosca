package pipeline

import (
	"context"
	"time"
)

type Runner interface {
	Run(ctx context.Context, req RunRequest) (*RunResult, error)
	RunStream(ctx context.Context, req RunRequest) (<-chan RunEvent, error)
}

type RunRequest struct {
	Prompt  string
	Agent   string
	History []Message
	Options RunOptions
}

type RunOptions struct {
	MaxTurns    int
	Temperature float64
	EnableBuild bool
	EnableTest  bool
	Timeout     time.Duration
	Planner     *Planner
}

type RunResult struct {
	Response    string
	Agent       string
	TokenUsage  TokenUsage
	TurnCount   int
	BuildResult *BuildResult
	TestResult  *TestResult
	TraceID     string
	MemoryID    string
	SkillsUsed  []string
}

type RunEvent struct {
	Type RunEventType
	Data any
}

type RunEventType string

const (
	EventContent    RunEventType = "content"
	EventToolStart  RunEventType = "tool_start"
	EventToolResult RunEventType = "tool_result"
	EventBuildStart RunEventType = "build_start"
	EventBuildEnd   RunEventType = "build_end"
	EventTestStart  RunEventType = "test_start"
	EventTestEnd    RunEventType = "test_end"
	EventError      RunEventType = "error"
	EventDone       RunEventType = "done"
)

type Message struct {
	Role    string
	Content string
}

type TokenUsage struct {
	Input  int
	Output int
}

type BuildResult struct {
	Success    bool
	Output     string
	DurationMs int64
}

type TestResult struct {
	Success    bool
	Passed     int
	Failed     int
	Output     string
	DurationMs int64
}
