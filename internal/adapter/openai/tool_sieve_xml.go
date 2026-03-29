package openai

import (
	"regexp"
	"strings"

	"ds2api/internal/util"
)

// --- XML tool call support for the streaming sieve ---

var xmlToolCallClosingTags = []string{"</tool_calls>", "</tool_call>", "</invoke>", "</function_call>", "</function_calls>", "</tool_use>"}
var xmlToolCallOpeningTags = []string{"<tool_calls", "<tool_call", "<invoke", "<function_call", "<function_calls", "<tool_use"}

// xmlToolCallBlockPattern matches a complete XML tool call block (wrapper or standalone).
var xmlToolCallBlockPattern = regexp.MustCompile(`(?is)(<tool_calls>\s*(?:.*?)\s*</tool_calls>|<tool_call>\s*(?:.*?)\s*</tool_call>|<invoke\b[^>]*>(?:.*?)</invoke>|<function_calls?\b[^>]*>(?:.*?)</function_calls?>|<tool_use>(?:.*?)</tool_use>)`)

// xmlToolTagsToDetect is the set of XML tag prefixes used by findToolSegmentStart.
var xmlToolTagsToDetect = []string{"<tool_calls>", "<tool_calls\n", "<tool_call>", "<tool_call\n",
	"<invoke ", "<invoke>", "<function_call", "<function_calls", "<tool_use>"}

// consumeXMLToolCapture tries to extract complete XML tool call blocks from captured text.
func consumeXMLToolCapture(captured string, toolNames []string) (prefix string, calls []util.ParsedToolCall, suffix string, ready bool) {
	lower := strings.ToLower(captured)
	// Find the earliest XML tool opening tag.
	openIdx := -1
	for _, tag := range xmlToolCallOpeningTags {
		idx := strings.Index(lower, tag)
		if idx >= 0 && (openIdx < 0 || idx < openIdx) {
			openIdx = idx
		}
	}
	if openIdx < 0 {
		return "", nil, "", false
	}

	// Look for a matching closing tag.
	closeIdx := -1
	for _, tag := range xmlToolCallClosingTags {
		idx := strings.Index(lower[openIdx:], tag)
		if idx >= 0 {
			absEnd := openIdx + idx + len(tag)
			if closeIdx < 0 || absEnd > closeIdx {
				closeIdx = absEnd
			}
		}
	}
	if closeIdx <= 0 {
		return "", nil, "", false
	}

	xmlBlock := captured[openIdx:closeIdx]
	prefixPart := captured[:openIdx]
	suffixPart := captured[closeIdx:]
	parsed := util.ParseToolCalls(xmlBlock, toolNames)
	if len(parsed) > 0 {
		prefixPart, suffixPart = trimWrappingJSONFence(prefixPart, suffixPart)
		return prefixPart, parsed, suffixPart, true
	}
	// Looks like XML tool syntax but failed to parse — consume it to avoid leak.
	return prefixPart, nil, suffixPart, true
}

// hasOpenXMLToolTag returns true if captured text contains an XML tool opening tag
// but no corresponding closing tag yet.
func hasOpenXMLToolTag(captured string) bool {
	lower := strings.ToLower(captured)
	for _, tag := range xmlToolCallOpeningTags {
		if strings.Contains(lower, tag) {
			hasClosed := false
			for _, ct := range xmlToolCallClosingTags {
				if strings.Contains(lower, ct) {
					hasClosed = true
					break
				}
			}
			if !hasClosed {
				return true
			}
		}
	}
	return false
}

// findPartialXMLToolTagStart checks if the string ends with a partial XML tool tag
// (e.g., "<tool_ca" or "<inv") and returns the position of the '<'.
func findPartialXMLToolTagStart(s string) int {
	lastLT := strings.LastIndex(s, "<")
	if lastLT < 0 {
		return -1
	}
	tail := s[lastLT:]
	// If there's a '>' in the tail, the tag is closed — not partial.
	if strings.Contains(tail, ">") {
		return -1
	}
	lowerTail := strings.ToLower(tail)
	// Check if the tail is a prefix of any known XML tool tag.
	for _, tag := range xmlToolCallOpeningTags {
		tagWithLT := tag
		if !strings.HasPrefix(tagWithLT, "<") {
			tagWithLT = "<" + tagWithLT
		}
		if strings.HasPrefix(tagWithLT, lowerTail) {
			return lastLT
		}
	}
	return -1
}
