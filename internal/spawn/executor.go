package spawn

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// TokenUsage holds token consumption from a Claude execution.
type TokenUsage struct {
	InputTokens         int64 `json:"input_tokens"`
	OutputTokens        int64 `json:"output_tokens"`
	CacheReadTokens     int64 `json:"cache_read_tokens"`
	CacheCreationTokens int64 `json:"cache_creation_tokens"`
}

// Total returns the sum of all token types.
func (t TokenUsage) Total() int64 {
	return t.InputTokens + t.OutputTokens + t.CacheReadTokens + t.CacheCreationTokens
}

// ExecResult holds the output of a claude execution.
type ExecResult struct {
	Stdout   string
	Stderr   string
	Duration time.Duration
	ExitCode int
	Err      error
	Tokens   TokenUsage
}

// MCPServer describes an MCP server to pass to the spawned claude process.
type MCPServer struct {
	Name    string
	Command string
	Args    []string
	Env     map[string]string
}

// Executor runs claude as a subprocess.
type Executor struct {
	ClaudeBinary string
	Logger       *slog.Logger
}

// NewExecutor creates an executor.
func NewExecutor(claudeBinary string, logger *slog.Logger) *Executor {
	return &Executor{
		ClaudeBinary: claudeBinary,
		Logger:       logger,
	}
}

// SpawnParams holds parameters for spawning a child agent.
type SpawnParams struct {
	Prompt       string
	TTL          time.Duration
	AllowedTools string
	MCPServers   []MCPServer
	Streaming    bool
}

// Run executes a claude subprocess with the given prompt and parameters.
func (e *Executor) Run(ctx context.Context, params SpawnParams) *ExecResult {
	args := e.buildArgs(params)

	e.Logger.Info("spawning child", "binary", e.ClaudeBinary)

	start := time.Now()
	execCtx, cancel := context.WithTimeout(ctx, params.TTL)
	defer cancel()

	cmd := exec.CommandContext(execCtx, e.ClaudeBinary, args...)
	cmd.Env = cleanEnv()
	cmd.Stdin = strings.NewReader(params.Prompt)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	duration := time.Since(start)

	result := &ExecResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		Duration: duration,
		Err:      err,
		ExitCode: exitCode(err),
		Tokens:   parseTokenUsage(stdout.String()),
	}

	if err != nil {
		e.Logger.Error("child failed", "duration", duration, "error", err)
	} else {
		e.Logger.Info("child completed", "duration", duration,
			"input_tokens", result.Tokens.InputTokens, "output_tokens", result.Tokens.OutputTokens)
	}

	return result
}

// RunWithLive executes with live output streaming.
func (e *Executor) RunWithLive(ctx context.Context, params SpawnParams, live io.Writer) *ExecResult {
	args := e.buildArgs(params)

	e.Logger.Info("spawning child (live)", "binary", e.ClaudeBinary)

	start := time.Now()
	execCtx, cancel := context.WithTimeout(ctx, params.TTL)
	defer cancel()

	cmd := exec.CommandContext(execCtx, e.ClaudeBinary, args...)
	cmd.Env = cleanEnv()
	cmd.Stdin = strings.NewReader(params.Prompt)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = io.MultiWriter(&stdout, live)
	cmd.Stderr = io.MultiWriter(&stderr, live)

	err := cmd.Run()
	duration := time.Since(start)

	result := &ExecResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		Duration: duration,
		Err:      err,
		ExitCode: exitCode(err),
		Tokens:   parseTokenUsage(stdout.String()),
	}

	if err != nil {
		e.Logger.Error("child failed", "duration", duration, "error", err)
	} else {
		e.Logger.Info("child completed", "duration", duration,
			"input_tokens", result.Tokens.InputTokens, "output_tokens", result.Tokens.OutputTokens)
	}

	return result
}

// IsClaudeAvailable checks if the claude binary is on PATH.
func (e *Executor) IsClaudeAvailable() bool {
	_, err := exec.LookPath(e.ClaudeBinary)
	return err == nil
}

func (e *Executor) buildArgs(params SpawnParams) []string {
	allowedTools := params.AllowedTools
	if allowedTools == "" {
		allowedTools = DefaultAllowedTools()
	}

	args := []string{"-p", "-"}
	if params.Streaming {
		args = append(args, "--output-format", "stream-json", "--verbose")
	}
	args = append(args, "--allowedTools", allowedTools)

	mcpConfig := buildMCPConfig(params.MCPServers)
	if mcpConfig != "" {
		args = append(args, "--mcp-config", mcpConfig)
	}

	return args
}

// DefaultAllowedTools returns the default allowed tools for spawned agents.
func DefaultAllowedTools() string {
	return "Agent,mcp__agent-relay__*,Bash,Read,Write,Edit,Grep,Glob"
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr.ExitCode()
	}
	return -1
}

func cleanEnv() []string {
	var env []string
	for _, e := range os.Environ() {
		if !strings.HasPrefix(e, "CLAUDECODE=") {
			env = append(env, e)
		}
	}
	return env
}

func buildMCPConfig(servers []MCPServer) string {
	if len(servers) == 0 {
		return ""
	}

	mcpJSON := map[string]any{
		"mcpServers": map[string]any{},
	}
	serversMap := mcpJSON["mcpServers"].(map[string]any)

	for _, s := range servers {
		entry := map[string]any{
			"command": s.Command,
		}
		if len(s.Args) > 0 {
			entry["args"] = s.Args
		}
		if len(s.Env) > 0 {
			entry["env"] = s.Env
		}
		serversMap[s.Name] = entry
	}

	tmpFile, err := os.CreateTemp("", "relay-mcp-*.json")
	if err != nil {
		return ""
	}
	defer tmpFile.Close()

	if err := json.NewEncoder(tmpFile).Encode(mcpJSON); err != nil {
		os.Remove(tmpFile.Name())
		return ""
	}

	return tmpFile.Name()
}

// parseTokenUsage extracts token usage from Claude stream-json or JSON output.
func parseTokenUsage(output string) TokenUsage {
	var usage TokenUsage
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var obj map[string]json.RawMessage
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			continue
		}
		if raw, ok := obj["usage"]; ok {
			var u struct {
				InputTokens         int64 `json:"input_tokens"`
				OutputTokens        int64 `json:"output_tokens"`
				CacheReadTokens     int64 `json:"cache_read_input_tokens"`
				CacheCreationTokens int64 `json:"cache_creation_input_tokens"`
			}
			if json.Unmarshal(raw, &u) == nil && (u.InputTokens > 0 || u.OutputTokens > 0) {
				usage.InputTokens = u.InputTokens
				usage.OutputTokens = u.OutputTokens
				usage.CacheReadTokens = u.CacheReadTokens
				usage.CacheCreationTokens = u.CacheCreationTokens
			}
		}
		if rawMsg, ok := obj["message"]; ok {
			var msg struct {
				Usage struct {
					InputTokens         int64 `json:"input_tokens"`
					OutputTokens        int64 `json:"output_tokens"`
					CacheReadTokens     int64 `json:"cache_read_input_tokens"`
					CacheCreationTokens int64 `json:"cache_creation_input_tokens"`
				} `json:"usage"`
			}
			if json.Unmarshal(rawMsg, &msg) == nil && (msg.Usage.InputTokens > 0 || msg.Usage.OutputTokens > 0) {
				usage.InputTokens = msg.Usage.InputTokens
				usage.OutputTokens = msg.Usage.OutputTokens
				usage.CacheReadTokens = msg.Usage.CacheReadTokens
				usage.CacheCreationTokens = msg.Usage.CacheCreationTokens
			}
		}
	}
	return usage
}

// LoadPromptFile reads a prompt from a file path (supports ~ expansion).
func LoadPromptFile(path string) (string, error) {
	expanded := path
	if len(path) > 1 && path[:2] == "~/" {
		home, err := os.UserHomeDir()
		if err == nil {
			expanded = filepath.Join(home, path[2:])
		}
	}
	data, err := os.ReadFile(expanded)
	if err != nil {
		return "", fmt.Errorf("reading prompt file %s: %w", expanded, err)
	}
	return string(data), nil
}
