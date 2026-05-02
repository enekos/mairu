package agent

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"mairu/internal/llm"
)

// mockBenchProvider is a lightweight provider for benchmarking.
type mockBenchProvider struct {
	history      []llm.Message
	isNew        bool
	systemPrompt string
	model        string
	tools        []llm.Tool
}

func newMockBenchProvider() *mockBenchProvider {
	return &mockBenchProvider{
		isNew: true,
		model: "mock-model",
		tools: []llm.Tool{
			{Name: "bash", Description: "run shell commands"},
			{Name: "read_file", Description: "read files"},
			{Name: "review_work", Description: "review work"},
		},
	}
}

func (m *mockBenchProvider) Chat(ctx context.Context, prompt string) (*llm.ChatResponse, error) {
	return &llm.ChatResponse{Content: "mock", FinishReason: "stop"}, nil
}
func (m *mockBenchProvider) ChatStream(ctx context.Context, prompt string) (llm.ChatStreamIterator, error) {
	m.history = append(m.history, llm.Message{Role: "user", Content: prompt})
	return &mockBenchStreamIterator{chunks: []llm.ChatStreamChunk{{Content: "ok", FinishReason: "stop"}}}, nil
}
func (m *mockBenchProvider) SendFunctionResponseStream(ctx context.Context, name string, result map[string]any) llm.ChatStreamIterator {
	return &mockBenchStreamIterator{chunks: []llm.ChatStreamChunk{{Content: "done", FinishReason: "stop"}}}
}
func (m *mockBenchProvider) SendFunctionResponsesStream(ctx context.Context, responses []llm.FunctionResponsePayload) llm.ChatStreamIterator {
	return &mockBenchStreamIterator{chunks: []llm.ChatStreamChunk{{Content: "done", FinishReason: "stop"}}}
}
func (m *mockBenchProvider) GenerateJSON(ctx context.Context, system, user string, schema *llm.JSONSchema, out any) error {
	return errors.New("not implemented")
}
func (m *mockBenchProvider) GenerateContent(ctx context.Context, model, prompt string) (string, error) {
	return `{"tools":["bash","read_file"]}`, nil
}
func (m *mockBenchProvider) SetSystemInstruction(prompt string) { m.systemPrompt = prompt }
func (m *mockBenchProvider) SetModel(modelName string)          { m.model = modelName }
func (m *mockBenchProvider) GetModelName() string               { return m.model }
func (m *mockBenchProvider) GetHistory() []llm.Message {
	return append([]llm.Message(nil), m.history...)
}
func (m *mockBenchProvider) SetHistory(history []llm.Message) {
	m.history = append([]llm.Message(nil), history...)
	m.isNew = false
}
func (m *mockBenchProvider) IsNewSession() bool                    { return m.isNew }
func (m *mockBenchProvider) GetTools() []llm.Tool                  { return append([]llm.Tool(nil), m.tools...) }
func (m *mockBenchProvider) SetTools(tools []llm.Tool)             {}
func (m *mockBenchProvider) RegisterDynamicTools(tools []llm.Tool) {}
func (m *mockBenchProvider) Close() error                          { return nil }

type mockBenchStreamIterator struct {
	chunks []llm.ChatStreamChunk
	idx    int
}

func (m *mockBenchStreamIterator) Next() (llm.ChatStreamChunk, error) {
	if m.idx >= len(m.chunks) {
		return llm.ChatStreamChunk{}, errors.New("EOF")
	}
	c := m.chunks[m.idx]
	m.idx++
	return c, nil
}
func (m *mockBenchStreamIterator) Done() bool {
	return m.idx >= len(m.chunks)
}

// BenchmarkAgentRunStream measures the full RunStream overhead (no real LLM).
func BenchmarkAgentRunStream(b *testing.B) {
	mock := newMockBenchProvider()
	ag, err := NewWithProvider(b.TempDir(), mock, Config{
		AgentSystemData: map[string]any{"CliHelp": ""},
	})
	if err != nil {
		b.Fatal(err)
	}
	ag.SetCouncilEnabled(false)
	prompt := "say hello"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		outChan := make(chan AgentEvent, 100)
		go ag.RunStream(prompt, outChan)
		for range outChan {
		}
	}
}

// BenchmarkTruncateHeadLarge measures truncation of large content.
func BenchmarkTruncateHeadLarge(b *testing.B) {
	content := strings.Repeat("this is a test line with some content\n", 10000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		TruncateHead(content, 2000, 50*1024)
	}
}

// BenchmarkTruncateTailLarge measures tail truncation of large content.
func BenchmarkTruncateTailLarge(b *testing.B) {
	content := strings.Repeat("this is a test line with some content\n", 10000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		TruncateTail(content, 2000, 50*1024)
	}
}

// BenchmarkStuckDetector measures stuck-detector overhead per tool call.
func BenchmarkStuckDetector(b *testing.B) {
	d := NewStuckDetector()
	args := map[string]any{"command": "echo hello", "file_path": "foo.go"}
	sig := NewToolSignature("bash", args)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		d.Record(sig)
		d.Check()
	}
}

// BenchmarkExtractFileOps measures compaction file-op extraction.
func BenchmarkExtractFileOps(b *testing.B) {
	var history []llm.Message
	for i := 0; i < 100; i++ {
		history = append(history, llm.Message{
			Role: "assistant",
			ToolCalls: []llm.ToolCall{
				{Name: "read_file", Arguments: map[string]any{"file_path": fmt.Sprintf("file%d.go", i)}},
				{Name: "write_file", Arguments: map[string]any{"file_path": fmt.Sprintf("out%d.go", i)}},
			},
		})
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = extractFileOps(history)
	}
}

// BenchmarkReadFile measures the actual ReadFile function with line formatting.
func BenchmarkReadFile(b *testing.B) {
	lines := make([]string, 2000)
	for i := range lines {
		lines[i] = fmt.Sprintf("line %d has some content here", i)
	}
	content := strings.Join(lines, "\n")

	tmpDir := b.TempDir()
	ag, err := NewWithProvider(tmpDir, newMockBenchProvider(), Config{})
	if err != nil {
		b.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "test.go"), []byte(content), 0644); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ag.ReadFile("test.go", 1, 2000)
	}
}

// BenchmarkStripANSI measures ANSI stripping overhead.
func BenchmarkStripANSI(b *testing.B) {
	s := "\x1b[31mred\x1b[0m \x1b[32mgreen\x1b[0m " + strings.Repeat("normal text ", 100)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = StripANSI(s)
	}
}

// BenchmarkPlannerHeuristic measures the planner's lightweight heuristic.
func BenchmarkPlannerHeuristic(b *testing.B) {
	prompt := "First refactor the auth module and then implement the new token validation logic, after that update every test file to match the new API, and finally write a summary of what changed."
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = isComplexPrompt(prompt)
	}
}

// BenchmarkAgentToolCallParallel measures parallel tool execution overhead.
func BenchmarkAgentToolCallParallel(b *testing.B) {
	mock := newMockBenchProvider()
	ag, err := NewWithProvider(b.TempDir(), mock, Config{
		AgentSystemData: map[string]any{"CliHelp": ""},
	})
	if err != nil {
		b.Fatal(err)
	}

	toolCalls := []llm.ToolCall{
		{ID: "t1", Name: "read_file", Arguments: map[string]any{"file_path": "a.go"}},
		{ID: "t2", Name: "read_file", Arguments: map[string]any{"file_path": "b.go"}},
		{ID: "t3", Name: "read_file", Arguments: map[string]any{"file_path": "c.go"}},
	}

	outChan := make(chan AgentEvent, 1000)
	go func() {
		for range outChan {
		}
	}()

	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var wg sync.WaitGroup
		results := make([]llm.FunctionResponsePayload, len(toolCalls))
		for j, tc := range toolCalls {
			wg.Add(1)
			go func(idx int, call llm.ToolCall) {
				defer wg.Done()
				res := ag.executeToolCall(ctx, call, outChan)
				results[idx] = llm.FunctionResponsePayload{
					Name:       call.Name,
					ToolCallID: call.ID,
					Response:   res,
				}
			}(j, tc)
		}
		wg.Wait()
	}
	b.StopTimer()
	close(outChan)
}

// BenchmarkAgentTurn simulates a realistic agent turn: file read + bash output
// with truncation, stuck detection, and event emission. More stable than
// BenchmarkAgentRunStream because it avoids mock-LLM planner variance.
func BenchmarkAgentTurn(b *testing.B) {
	// Realistic large outputs
	fileContent := strings.Repeat("this is a source code line with some content here\n", 5000)
	bashContent := strings.Repeat("build log line with compilation output and warnings\n", 5000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Simulate file read truncation (head)
		_ = TruncateHead(fileContent, 2000, 50*1024)

		// Simulate bash output truncation (tail)
		_ = TruncateTail(bashContent, 2000, 50*1024)

		// Simulate stuck detection
		d := NewStuckDetector()
		for j := 0; j < 5; j++ {
			d.Record(NewToolSignature("bash", map[string]any{"command": fmt.Sprintf("echo %d", j)}))
			d.Check()
		}

		// Simulate event emission
		outChan := make(chan AgentEvent, 100)
		go func() {
			for range outChan {
			}
		}()
		outChan <- AgentEvent{Type: "text", Content: "ok"}
		close(outChan)
	}
}

// BenchmarkEventChannelThroughput measures raw channel send/receive throughput.
func BenchmarkEventChannelThroughput(b *testing.B) {
	outChan := make(chan AgentEvent, 1000)
	done := make(chan struct{})
	go func() {
		for range outChan {
		}
		close(done)
	}()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		outChan <- AgentEvent{Type: "text", Content: "hello world"}
	}
	close(outChan)
	<-done
}
