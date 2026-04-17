// Package parser provides streaming JSONL parsing for Claude and Codex transcripts.
package parser

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/boshu2/agentops/cli/internal/types"
)

// DefaultMaxContentLength is the default truncation limit for content fields.
const DefaultMaxContentLength = 500

// Message type constants for transcript entries.
const (
	msgTypeUser       = "user"
	msgTypeAssistant  = "assistant"
	msgTypeToolUse    = "tool_use"
	msgTypeToolResult = "tool_result"
)

// Error classification constants for parse errors.
const (
	errClassJSON     = "json"
	errClassSchema   = "schema"
	errClassEncoding = "encoding"
)

// Parser handles streaming JSONL parsing with configurable options.
type Parser struct {
	// MaxContentLength is the maximum characters before truncation.
	MaxContentLength int

	// SkipMalformed skips malformed lines instead of erroring.
	SkipMalformed bool

	// OnProgress is called with progress updates for large files.
	OnProgress func(linesProcessed, totalLines int)
}

// NewParser creates a parser with default settings.
func NewParser() *Parser {
	return &Parser{
		MaxContentLength: DefaultMaxContentLength,
		SkipMalformed:    true,
	}
}

// rawMessage represents the raw JSON structure from supported transcript surfaces.
type rawMessage struct {
	Type       string          `json:"type"`
	SessionID  string          `json:"sessionId"`
	Timestamp  string          `json:"timestamp"`
	UUID       string          `json:"uuid"`
	ParentUUID string          `json:"parentUuid,omitempty"`
	Role       string          `json:"role,omitempty"`
	Content    any             `json:"content,omitempty"`
	ToolName   string          `json:"tool_name,omitempty"`
	ToolInput  map[string]any  `json:"tool_input,omitempty"`
	ToolOutput any             `json:"tool_output,omitempty"`
	Payload    json.RawMessage `json:"payload,omitempty"`
	Message    *struct {
		Type    string `json:"type,omitempty"`
		Role    string `json:"role"`
		Content any    `json:"content"` // Can be string or array
		Tools   []struct {
			Name   string         `json:"name"`
			Input  map[string]any `json:"input"`
			Output any            `json:"output"`
		} `json:"tools,omitempty"`
	} `json:"message,omitempty"`
	// ToolUseResult contains structured tool output (e.g., for TodoWrite)
	ToolUseResult any `json:"toolUseResult,omitempty"`
}

type codexSessionMeta struct {
	ID        string `json:"id"`
	Timestamp string `json:"timestamp"`
}

type codexEventPayload struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

type codexResponseItem struct {
	Type      string `json:"type"`
	Role      string `json:"role"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
	Output    string `json:"output"`
	Content   []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
}

// ParseResult contains the result of parsing a JSONL stream.
type ParseResult struct {
	Messages       []types.TranscriptMessage
	TotalLines     int
	MalformedLines int
	Errors         []error

	// Checksum is SHA256 hash of the parsed content (first 16 hex chars).
	// Used for detecting changes and validating re-parsing.
	Checksum string

	// FilePath is the source file path (if parsed from file).
	FilePath string

	// ParsedAt is when parsing completed.
	ParsedAt time.Time
}

// ParseError provides structured error information for transcript parsing failures.
type ParseError struct {
	Line       int    `json:"line"`
	Column     int    `json:"column,omitempty"`
	Message    string `json:"message"`
	RawContent string `json:"raw_content,omitempty"`
	ErrorType  string `json:"error_type"` // "json", "schema", "encoding"
}

func (e *ParseError) Error() string {
	if e.Column > 0 {
		return fmt.Sprintf("line %d, col %d: %s (%s)", e.Line, e.Column, e.Message, e.ErrorType)
	}
	return fmt.Sprintf("line %d: %s (%s)", e.Line, e.Message, e.ErrorType)
}

// Parse reads JSONL from the reader and returns parsed messages.
func (p *Parser) Parse(r io.Reader) (*ParseResult, error) {
	result := &ParseResult{
		Messages: make([]types.TranscriptMessage, 0),
	}

	hasher := sha256.New()
	if err := readJSONLLines(r, func(line []byte, lineNum int) error {
		result.TotalLines = lineNum

		if len(line) == 0 {
			return nil
		}

		_, _ = hasher.Write(line)
		_, _ = hasher.Write([]byte("\n"))

		p.processLine(line, lineNum, result)

		if p.OnProgress != nil && lineNum%100 == 0 {
			p.OnProgress(lineNum, 0)
		}
		return nil
	}); err != nil {
		return result, fmt.Errorf("read jsonl: %w", err)
	}

	hash := hasher.Sum(nil)
	result.Checksum = hex.EncodeToString(hash[:8])
	result.ParsedAt = time.Now()

	return result, nil
}

// processLine parses a single JSONL line and appends the result or error.
func (p *Parser) processLine(line []byte, lineNum int, result *ParseResult) {
	msg, err := p.parseLine(line, lineNum)
	if err != nil {
		result.MalformedLines++
		if !p.SkipMalformed {
			result.Errors = append(result.Errors, &ParseError{
				Line:       lineNum,
				Message:    err.Error(),
				ErrorType:  classifyError(err),
				RawContent: truncateForError(string(line), 100),
			})
		}
		return
	}
	if msg != nil {
		result.Messages = append(result.Messages, *msg)
	}
}

// classifyError determines the error type for structured reporting.
func classifyError(err error) string {
	errStr := err.Error()
	switch {
	case strings.Contains(errStr, "invalid character"):
		return errClassJSON
	case strings.Contains(errStr, "unexpected end"):
		return errClassJSON
	case strings.Contains(errStr, "cannot unmarshal"):
		return errClassSchema
	case strings.Contains(errStr, "invalid UTF-8"):
		return errClassEncoding
	default:
		return errClassJSON
	}
}

// truncateForError limits error context to a reasonable size.
func truncateForError(s string, maxLen int) string {
	// Fast path: len(s) is an upper bound on rune count, so any string whose
	// byte length already fits needs no conversion. This avoids a []rune
	// allocation on the common short-ASCII case.
	if len(s) <= maxLen {
		return s
	}
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}

// openFileFunc is the function used to open files for parsing.
// It can be overridden in tests to inject close errors.
var openFileFunc = func(path string) (io.ReadCloser, error) {
	return os.Open(path)
}

// ParseFile parses a JSONL file by path.
func (p *Parser) ParseFile(path string) (result *ParseResult, err error) {
	f, err := openFileFunc(path)
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	defer func() {
		if cerr := f.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	result, err = p.Parse(f)
	if result != nil {
		result.FilePath = path
	}
	return result, err
}

// timestampFormats lists the formats to try when parsing timestamps.
var timestampFormats = []string{
	time.RFC3339,
	"2006-01-02T15:04:05.000Z",
}

// parseTimestamp parses a timestamp string, trying multiple formats.
// Returns zero time if all formats fail.
func parseTimestamp(s string) time.Time {
	for _, format := range timestampFormats {
		if ts, err := time.Parse(format, s); err == nil {
			return ts
		}
	}
	return time.Time{}
}

// isValidMessageType checks if the type is one we should parse.
func isValidMessageType(msgType string) bool {
	switch msgType {
	case msgTypeUser, msgTypeAssistant, msgTypeToolUse, msgTypeToolResult:
		return true
	default:
		return false
	}
}

// parseContentBlocks extracts text and tool calls from content block array.
func (p *Parser) parseContentBlocks(blocks []any) (string, []types.ToolCall) {
	var content string
	var tools []types.ToolCall

	for _, block := range blocks {
		blockMap, ok := block.(map[string]any)
		if !ok {
			continue
		}
		text, tool := p.classifyBlock(blockMap)
		content += text
		if tool != nil {
			tools = append(tools, *tool)
		}
	}

	return content, tools
}

// classifyBlock dispatches a single content block into text or tool call.
func (p *Parser) classifyBlock(blockMap map[string]any) (string, *types.ToolCall) {
	blockType, _ := blockMap["type"].(string)
	switch blockType {
	case "text":
		if text, ok := blockMap["text"].(string); ok {
			return p.truncate(text), nil
		}
	case msgTypeToolUse:
		return "", p.parseToolUse(blockMap)
	case msgTypeToolResult:
		return "", p.parseToolResult(blockMap)
	}
	return "", nil
}

// parseLine parses a single JSON line.
func (p *Parser) parseLine(line []byte, lineNum int) (*types.TranscriptMessage, error) {
	var raw rawMessage
	if err := json.Unmarshal(line, &raw); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	switch raw.Type {
	case msgTypeUser, msgTypeAssistant:
		return p.parseClaudeMessage(raw, lineNum), nil
	case msgTypeToolUse:
		return p.parseClaudeToolUse(raw, lineNum), nil
	case msgTypeToolResult:
		return p.parseClaudeToolResult(raw, lineNum), nil
	case "session_meta":
		return p.parseCodexSessionMeta(raw, lineNum)
	case "event_msg":
		return p.parseCodexEvent(raw, lineNum)
	case "response_item":
		return p.parseCodexResponseItem(raw, lineNum)
	default:
		return nil, nil
	}
}

func (p *Parser) parseClaudeMessage(raw rawMessage, lineNum int) *types.TranscriptMessage {
	msg := &types.TranscriptMessage{
		Type:         raw.Type,
		Timestamp:    parseTimestamp(raw.Timestamp),
		SessionID:    raw.SessionID,
		MessageIndex: lineNum,
		Role:         raw.Role,
	}

	if raw.Message != nil {
		msg.Role = raw.Message.Role
		p.extractMessageContent(raw.Message.Content, msg)
	}
	if raw.Content != nil {
		p.extractMessageContent(raw.Content, msg)
	}
	if msg.Role == "" {
		msg.Role = raw.Type
	}

	return msg
}

func (p *Parser) parseClaudeToolUse(raw rawMessage, lineNum int) *types.TranscriptMessage {
	var tools []types.ToolCall
	if raw.ToolName != "" {
		tools = append(tools, types.ToolCall{
			Name:  raw.ToolName,
			Input: raw.ToolInput,
		})
	}
	if raw.Message != nil {
		for _, tool := range raw.Message.Tools {
			if tool.Name == "" {
				continue
			}
			tools = append(tools, types.ToolCall{
				Name:   tool.Name,
				Input:  tool.Input,
				Output: p.extractTopLevelToolOutput(tool.Output),
			})
		}
	}
	if len(tools) == 0 && raw.Message != nil {
		msg := &types.TranscriptMessage{
			Type:         msgTypeToolUse,
			Role:         raw.Message.Role,
			Timestamp:    parseTimestamp(raw.Timestamp),
			SessionID:    raw.SessionID,
			MessageIndex: lineNum,
		}
		p.extractMessageContent(raw.Message.Content, msg)
		return msg
	}
	if len(tools) == 0 {
		return nil
	}
	return &types.TranscriptMessage{
		Type:         msgTypeToolUse,
		Role:         msgTypeAssistant,
		Timestamp:    parseTimestamp(raw.Timestamp),
		SessionID:    raw.SessionID,
		MessageIndex: lineNum,
		Tools:        tools,
	}
}

func (p *Parser) parseClaudeToolResult(raw rawMessage, lineNum int) *types.TranscriptMessage {
	output := p.extractTopLevelToolOutput(raw.ToolOutput)
	if output == "" && raw.ToolUseResult != nil {
		output = p.extractTopLevelToolOutput(raw.ToolUseResult)
	}
	if output == "" && raw.Message != nil && raw.Message.Content != nil {
		output = p.extractTopLevelToolOutput(raw.Message.Content)
	}
	role := raw.Role
	if role == "" && raw.Message != nil {
		role = raw.Message.Role
	}
	return &types.TranscriptMessage{
		Type:         msgTypeToolResult,
		Role:         coalesce(role, msgTypeAssistant),
		Timestamp:    parseTimestamp(raw.Timestamp),
		SessionID:    raw.SessionID,
		MessageIndex: lineNum,
		Tools: []types.ToolCall{{
			Name:   coalesce(raw.ToolName, msgTypeToolResult),
			Input:  raw.ToolInput,
			Output: output,
		}},
	}
}

func (p *Parser) parseCodexSessionMeta(raw rawMessage, lineNum int) (*types.TranscriptMessage, error) {
	var meta codexSessionMeta
	if err := json.Unmarshal(raw.Payload, &meta); err != nil {
		return nil, fmt.Errorf("invalid session_meta payload: %w", err)
	}
	return &types.TranscriptMessage{
		Type:         "session_meta",
		Timestamp:    parseTimestamp(coalesce(meta.Timestamp, raw.Timestamp)),
		SessionID:    coalesce(meta.ID, raw.SessionID),
		MessageIndex: lineNum,
	}, nil
}

func (p *Parser) parseCodexEvent(raw rawMessage, lineNum int) (*types.TranscriptMessage, error) {
	var payload codexEventPayload
	if err := json.Unmarshal(raw.Payload, &payload); err != nil {
		return nil, fmt.Errorf("invalid event payload: %w", err)
	}

	var (
		msgType string
		role    string
	)
	switch payload.Type {
	case "user_message":
		msgType = msgTypeUser
		role = msgTypeUser
	case "agent_message":
		msgType = msgTypeAssistant
		role = msgTypeAssistant
	default:
		return nil, nil
	}

	return &types.TranscriptMessage{
		Type:         msgType,
		Role:         role,
		Content:      p.truncate(payload.Message),
		Timestamp:    parseTimestamp(raw.Timestamp),
		SessionID:    raw.SessionID,
		MessageIndex: lineNum,
	}, nil
}

func (p *Parser) parseCodexResponseItem(raw rawMessage, lineNum int) (*types.TranscriptMessage, error) {
	var item codexResponseItem
	if err := json.Unmarshal(raw.Payload, &item); err != nil {
		return nil, fmt.Errorf("invalid response_item payload: %w", err)
	}

	switch item.Type {
	case "message":
		if item.Role != msgTypeUser && item.Role != msgTypeAssistant {
			return nil, nil
		}
		var content strings.Builder
		for _, block := range item.Content {
			switch block.Type {
			case "input_text", "output_text", "text":
				content.WriteString(block.Text)
			}
		}
		return &types.TranscriptMessage{
			Type:         item.Role,
			Role:         item.Role,
			Content:      p.truncate(content.String()),
			Timestamp:    parseTimestamp(raw.Timestamp),
			SessionID:    raw.SessionID,
			MessageIndex: lineNum,
		}, nil
	case "function_call", "custom_tool_call":
		return &types.TranscriptMessage{
			Type:         msgTypeToolUse,
			Role:         msgTypeAssistant,
			Timestamp:    parseTimestamp(raw.Timestamp),
			SessionID:    raw.SessionID,
			MessageIndex: lineNum,
			Tools: []types.ToolCall{{
				Name:  coalesce(item.Name, item.Type),
				Input: parseCodexToolInput(item.Arguments),
			}},
		}, nil
	case "function_call_output", "custom_tool_call_output":
		return &types.TranscriptMessage{
			Type:         msgTypeToolResult,
			Role:         msgTypeAssistant,
			Timestamp:    parseTimestamp(raw.Timestamp),
			SessionID:    raw.SessionID,
			MessageIndex: lineNum,
			Tools: []types.ToolCall{{
				Name:   msgTypeToolResult,
				Output: p.truncate(item.Output),
			}},
		}, nil
	default:
		return nil, nil
	}
}

// extractMessageContent populates msg.Content and msg.Tools from raw content.
func (p *Parser) extractMessageContent(rawContent any, msg *types.TranscriptMessage) {
	switch content := rawContent.(type) {
	case string:
		msg.Content = p.truncate(content)
	case []any:
		msg.Content, msg.Tools = p.parseContentBlocks(content)
	}
}

// parseToolUse extracts tool call information from a tool_use block.
func (p *Parser) parseToolUse(block map[string]any) *types.ToolCall {
	name, _ := block["name"].(string)
	if name == "" {
		return nil
	}

	toolCall := &types.ToolCall{
		Name: name,
	}

	// Extract input parameters
	if input, ok := block["input"].(map[string]any); ok {
		toolCall.Input = input
	}

	return toolCall
}

// parseToolResult extracts tool result information from a tool_result block.
func (p *Parser) parseToolResult(block map[string]any) *types.ToolCall {
	toolCall := &types.ToolCall{
		Name: msgTypeToolResult,
	}

	// Check if it's an error result
	if isError, ok := block["is_error"].(bool); ok && isError {
		toolCall.Error = "tool error"
	}

	toolCall.Output = p.extractToolResultContent(block["content"])
	return toolCall
}

// extractToolResultContent extracts text from a tool_result content field,
// which may be a plain string or an array of text blocks.
func (p *Parser) extractToolResultContent(content any) string {
	switch c := content.(type) {
	case string:
		return p.truncate(c)
	case []any:
		var out string
		for _, item := range c {
			if itemMap, ok := item.(map[string]any); ok {
				if text, ok := itemMap["text"].(string); ok {
					out += p.truncate(text)
				}
			}
		}
		return out
	default:
		return ""
	}
}

func (p *Parser) extractTopLevelToolOutput(content any) string {
	switch c := content.(type) {
	case nil:
		return ""
	case string:
		return p.truncate(c)
	case []any:
		return p.extractToolResultContent(c)
	default:
		data, err := json.Marshal(c)
		if err != nil {
			return ""
		}
		return p.truncate(string(data))
	}
}

// truncate limits content to MaxContentLength characters.
// Slices at rune boundaries to avoid splitting multi-byte UTF-8 sequences.
func (p *Parser) truncate(s string) string {
	if p.MaxContentLength <= 0 {
		return s
	}
	// Fast path: byte length is an upper bound on rune count.
	if len(s) <= p.MaxContentLength {
		return s
	}
	runes := []rune(s)
	if len(runes) <= p.MaxContentLength {
		return s
	}
	return string(runes[:p.MaxContentLength]) + "... [truncated]"
}

// ParseChannel returns a channel that emits messages as they're parsed.
// Useful for streaming large files without loading all into memory.
func (p *Parser) ParseChannel(r io.Reader) (<-chan types.TranscriptMessage, <-chan error) {
	msgCh := make(chan types.TranscriptMessage, 100)
	errCh := make(chan error, 1)

	go func() {
		defer close(msgCh)
		defer close(errCh)
		p.channelScanner(r, msgCh, errCh)
	}()

	return msgCh, errCh
}

// processChannelLine parses one scanner line and forwards the result to msgCh/errCh.
// Returns false if scanning should stop (fatal parse error).
func (p *Parser) processChannelLine(line []byte, lineNum int, msgCh chan<- types.TranscriptMessage, errCh chan<- error) bool {
	if len(line) == 0 {
		return true
	}
	msg, err := p.parseLine(line, lineNum)
	if err != nil {
		if !p.SkipMalformed {
			errCh <- fmt.Errorf("line %d: %w", lineNum, err)
			return false
		}
		return true
	}
	if msg != nil {
		msgCh <- *msg
	}
	return true
}

// channelScanner scans r line by line, sending parsed messages to msgCh and errors to errCh.
func (p *Parser) channelScanner(r io.Reader, msgCh chan<- types.TranscriptMessage, errCh chan<- error) {
	if err := readJSONLLines(r, func(line []byte, lineNum int) error {
		if !p.processChannelLine(line, lineNum, msgCh, errCh) {
			return errStopChannelScan
		}
		return nil
	}); err != nil && !errors.Is(err, errStopChannelScan) {
		errCh <- fmt.Errorf("read jsonl: %w", err)
	}
}

var errStopChannelScan = errors.New("stop channel scan")

func readJSONLLines(r io.Reader, fn func([]byte, int) error) error {
	reader := bufio.NewReader(r)
	lineNum := 0

	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			lineNum++
			line = bytes.TrimSuffix(line, []byte("\n"))
			line = bytes.TrimSuffix(line, []byte("\r"))
			if callErr := fn(line, lineNum); callErr != nil {
				return callErr
			}
		}

		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
	}
}

func parseCodexToolInput(raw string) map[string]any {
	if strings.TrimSpace(raw) == "" {
		return nil
	}

	var data any
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		return map[string]any{"raw": raw}
	}
	if obj, ok := data.(map[string]any); ok {
		return obj
	}
	return map[string]any{"value": data}
}

func coalesce(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
