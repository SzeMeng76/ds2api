package openai

import (
	"strings"
	"testing"
)

func TestProcessToolSieveInterceptsXMLToolCallWithoutLeak(t *testing.T) {
	var state toolStreamSieveState
	// Simulate a model producing XML tool call output chunk by chunk.
	chunks := []string{
		"<tool_calls>\n",
		"  <tool_call>\n",
		"    <tool_name>read_file</tool_name>\n",
		`    <parameters>{"path":"README.MD"}</parameters>` + "\n",
		"  </tool_call>\n",
		"</tool_calls>",
	}
	var events []toolStreamEvent
	for _, c := range chunks {
		events = append(events, processToolSieveChunk(&state, c, []string{"read_file"})...)
	}
	events = append(events, flushToolSieve(&state, []string{"read_file"})...)

	var textContent string
	var toolCalls int
	for _, evt := range events {
		if evt.Content != "" {
			textContent += evt.Content
		}
		toolCalls += len(evt.ToolCalls)
	}

	if strings.Contains(textContent, "<tool_call") {
		t.Fatalf("XML tool call content leaked to text: %q", textContent)
	}
	if strings.Contains(textContent, "read_file") {
		t.Fatalf("tool name leaked to text: %q", textContent)
	}
	if toolCalls == 0 {
		t.Fatal("expected tool calls to be extracted, got none")
	}
}

func TestProcessToolSieveXMLWithLeadingText(t *testing.T) {
	var state toolStreamSieveState
	// Model outputs some prose then an XML tool call.
	chunks := []string{
		"Let me check the file.\n",
		"<tool_calls>\n  <tool_call>\n    <tool_name>read_file</tool_name>\n",
		`    <parameters>{"path":"go.mod"}</parameters>` + "\n  </tool_call>\n</tool_calls>",
	}
	var events []toolStreamEvent
	for _, c := range chunks {
		events = append(events, processToolSieveChunk(&state, c, []string{"read_file"})...)
	}
	events = append(events, flushToolSieve(&state, []string{"read_file"})...)

	var textContent string
	var toolCalls int
	for _, evt := range events {
		if evt.Content != "" {
			textContent += evt.Content
		}
		toolCalls += len(evt.ToolCalls)
	}

	// Leading text should be emitted.
	if !strings.Contains(textContent, "Let me check the file.") {
		t.Fatalf("expected leading text to be emitted, got %q", textContent)
	}
	// The XML itself should NOT leak.
	if strings.Contains(textContent, "<tool_call") {
		t.Fatalf("XML tool call content leaked to text: %q", textContent)
	}
	if toolCalls == 0 {
		t.Fatal("expected tool calls to be extracted, got none")
	}
}

func TestProcessToolSievePartialXMLTagHeldBack(t *testing.T) {
	var state toolStreamSieveState
	// Chunk ends with a partial XML tool tag.
	events := processToolSieveChunk(&state, "Hello <tool_ca", []string{"read_file"})

	var textContent string
	for _, evt := range events {
		textContent += evt.Content
	}

	// "Hello " should be emitted, but "<tool_ca" should be held back.
	if strings.Contains(textContent, "<tool_ca") {
		t.Fatalf("partial XML tag should not be emitted, got %q", textContent)
	}
	if !strings.Contains(textContent, "Hello") {
		t.Fatalf("expected 'Hello' text to be emitted, got %q", textContent)
	}
}

func TestFindToolSegmentStartDetectsXMLToolCalls(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  int
	}{
		{"tool_calls_tag", "some text <tool_calls>\n", 10},
		{"tool_call_tag", "prefix <tool_call>\n", 7},
		{"invoke_tag", "text <invoke name=\"foo\">body</invoke>", 5},
		{"function_call_tag", "<function_call name=\"foo\">body</function_call>", 0},
		{"no_xml", "just plain text", -1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := findToolSegmentStart(tc.input)
			if got != tc.want {
				t.Fatalf("findToolSegmentStart(%q) = %d, want %d", tc.input, got, tc.want)
			}
		})
	}
}

func TestFindPartialXMLToolTagStart(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  int
	}{
		{"partial_tool_call", "Hello <tool_ca", 6},
		{"partial_invoke", "Prefix <inv", 7},
		{"partial_lt_only", "Text <", 5},
		{"complete_tag", "Text <tool_call>done", -1},
		{"no_lt", "plain text", -1},
		{"closed_lt", "a < b > c", -1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := findPartialXMLToolTagStart(tc.input)
			if got != tc.want {
				t.Fatalf("findPartialXMLToolTagStart(%q) = %d, want %d", tc.input, got, tc.want)
			}
		})
	}
}

func TestHasOpenXMLToolTag(t *testing.T) {
	if !hasOpenXMLToolTag("<tool_call>\n<tool_name>foo</tool_name>") {
		t.Fatal("should detect open XML tool tag without closing tag")
	}
	if hasOpenXMLToolTag("<tool_call>\n<tool_name>foo</tool_name></tool_call>") {
		t.Fatal("should return false when closing tag is present")
	}
	if hasOpenXMLToolTag("plain text without any XML") {
		t.Fatal("should return false for plain text")
	}
}
