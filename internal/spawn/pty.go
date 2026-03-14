package spawn

import (
	"context"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/creack/pty"
)

const replayBufSize = 128 * 1024 // 128KB rolling buffer

// PTYSession holds a running process with a PTY attached.
// A persistent reader goroutine pumps PTY output into a ring buffer
// and broadcasts to subscribed WebSocket connections.
type PTYSession struct {
	ID     string
	PTY    *os.File
	Cmd    *exec.Cmd
	Cancel context.CancelFunc

	mu     sync.Mutex
	closed bool

	// Ring buffer for replay on reconnect
	ring    []byte
	ringPos int  // next write position in ring
	ringLen int  // bytes written (capped at replayBufSize)

	// Subscriber channels — each WS connection gets one
	subs   map[uint64]chan []byte
	subSeq uint64
	subMu  sync.Mutex

	// Signals that the PTY reader has stopped
	done chan struct{}
}

// Write sends input to the PTY (from browser → process stdin).
func (s *PTYSession) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return 0, io.ErrClosedPipe
	}
	return s.PTY.Write(p)
}

// Close terminates the PTY session.
func (s *PTYSession) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.closed = true
	s.Cancel()
	s.PTY.Close()
}

// Resize changes the PTY window size.
func (s *PTYSession) Resize(rows, cols uint16) error {
	return pty.Setsize(s.PTY, &pty.Winsize{Rows: rows, Cols: cols})
}

// Subscribe returns the replay buffer contents and a channel for live output.
// Call Unsubscribe when the WS disconnects.
func (s *PTYSession) Subscribe() (replay []byte, ch <-chan []byte, id uint64) {
	s.subMu.Lock()
	defer s.subMu.Unlock()

	// Snapshot the ring buffer
	s.mu.Lock()
	if s.ringLen > 0 {
		replay = make([]byte, s.ringLen)
		if s.ringLen < replayBufSize {
			copy(replay, s.ring[:s.ringLen])
		} else {
			// Ring wrapped — read from ringPos to end, then start to ringPos
			n := copy(replay, s.ring[s.ringPos:])
			copy(replay[n:], s.ring[:s.ringPos])
		}
	}
	s.mu.Unlock()

	c := make(chan []byte, 64)
	s.subSeq++
	id = s.subSeq
	s.subs[id] = c
	return replay, c, id
}

// Unsubscribe removes a subscriber.
func (s *PTYSession) Unsubscribe(id uint64) {
	s.subMu.Lock()
	defer s.subMu.Unlock()
	if ch, ok := s.subs[id]; ok {
		close(ch)
		delete(s.subs, id)
	}
}

// Done returns a channel that closes when the PTY reader stops (process exit).
func (s *PTYSession) Done() <-chan struct{} {
	return s.done
}

// startReader pumps PTY output into the ring buffer and broadcasts to subscribers.
// Must be called once after PTY is started.
func (s *PTYSession) startReader() {
	s.done = make(chan struct{})
	s.subs = make(map[uint64]chan []byte)
	s.ring = make([]byte, replayBufSize)

	go func() {
		defer close(s.done)
		buf := make([]byte, 4096)
		for {
			n, err := s.PTY.Read(buf)
			if n > 0 {
				chunk := make([]byte, n)
				copy(chunk, buf[:n])

				// Write to ring buffer
				s.mu.Lock()
				for i := 0; i < n; i++ {
					s.ring[s.ringPos] = chunk[i]
					s.ringPos = (s.ringPos + 1) % replayBufSize
					if s.ringLen < replayBufSize {
						s.ringLen++
					}
				}
				s.mu.Unlock()

				// Broadcast to subscribers (non-blocking)
				s.subMu.Lock()
				for _, ch := range s.subs {
					select {
					case ch <- chunk:
					default:
						// subscriber too slow, drop
					}
				}
				s.subMu.Unlock()
			}
			if err != nil {
				return
			}
		}
	}()
}

// StartPTY spawns claude in INTERACTIVE mode with a pseudo-terminal.
// The context prompt is passed via --append-system-prompt so the agent
// boots with full identity/context. workDir sets the process cwd.
func (e *Executor) StartPTY(ctx context.Context, params SpawnParams, workDir string) (*PTYSession, error) {
	args := []string{}

	allowedTools := params.AllowedTools
	if allowedTools == "" {
		allowedTools = DefaultAllowedTools()
	}
	args = append(args, "--allowedTools", allowedTools)

	mcpConfig := buildMCPConfig(params.MCPServers)
	if mcpConfig != "" {
		args = append(args, "--mcp-config", mcpConfig)
	}

	if params.Prompt != "" {
		args = append(args, "--append-system-prompt", params.Prompt)
	}

	execCtx, cancel := context.WithTimeout(ctx, params.TTL)
	cmd := exec.CommandContext(execCtx, e.ClaudeBinary, args...)
	cmd.Env = ptyEnv()
	if workDir != "" {
		cmd.Dir = workDir
	}

	ptmx, err := pty.Start(cmd)
	if err != nil {
		cancel()
		return nil, err
	}

	_ = pty.Setsize(ptmx, &pty.Winsize{Rows: 24, Cols: 120})

	e.Logger.Info("spawned interactive PTY", "binary", e.ClaudeBinary, "pid", cmd.Process.Pid, "prompt_len", len(params.Prompt), "work_dir", workDir)

	sess := &PTYSession{
		PTY:    ptmx,
		Cmd:    cmd,
		Cancel: cancel,
	}
	sess.startReader()

	return sess, nil
}

// ptyEnv returns a clean environment with TERM set for TUI rendering.
func ptyEnv() []string {
	env := cleanEnv()
	hasTerm := false
	for i, e := range env {
		if strings.HasPrefix(e, "TERM=") {
			env[i] = "TERM=xterm-256color"
			hasTerm = true
			break
		}
	}
	if !hasTerm {
		env = append(env, "TERM=xterm-256color")
	}
	env = append(env, "FORCE_COLOR=1", "COLORTERM=truecolor")
	return env
}
