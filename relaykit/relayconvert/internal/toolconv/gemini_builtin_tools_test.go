package toolconv

import (
	"testing"

	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/relayconvert/convmeta"
	kitutil "github.com/QuantumNous/new-api/relaykit/relayconvert/kitutil"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Gemini rejects a request that pairs a built-in tool with function declarations
// unless toolConfig.includeServerSideToolInvocations is set:
// "Please enable tool_config.include_server_side_tool_invocations to use Built-in
// tools with Function calling." Codex sends exactly that combination.
func TestGeminiServerSideToolInvocationsFlag(t *testing.T) {
	t.Parallel()

	function := map[string]any{
		"type":        "function",
		"name":        "shell",
		"description": "run a command",
		"parameters":  map[string]any{"type": "object", "properties": map[string]any{}},
	}
	webSearch := map[string]any{"type": "web_search"}

	tests := []struct {
		name  string
		tools []map[string]any
		want  bool
	}{
		{name: "built-in with function calling", tools: []map[string]any{webSearch, function}, want: true},
		{name: "function calling only", tools: []map[string]any{function}, want: false},
		{name: "built-in only", tools: []map[string]any{webSearch}, want: false},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			tools, err := kitutil.Marshal(testCase.tools)
			require.NoError(t, err)
			source := &dto.OpenAIResponsesRequest{Model: "gemini-2.5-pro", Tools: tools}
			_, set, err := ExtractRequest(types.RelayFormatOpenAIResponses, source)
			require.NoError(t, err)

			target := &dto.GeminiChatRequest{
				Contents: []dto.GeminiChatContent{{Role: "user", Parts: []dto.GeminiPart{{Text: "hi"}}}},
			}
			out, _, err := AttachRequest(types.RelayFormatGemini, target, set, &convmeta.Options{})
			require.NoError(t, err)
			converted, ok := out.(*dto.GeminiChatRequest)
			require.True(t, ok)

			if !testCase.want {
				if converted.ToolConfig != nil {
					assert.Nil(t, converted.ToolConfig.IncludeServerSideToolInvocations)
				}
				return
			}
			require.NotNil(t, converted.ToolConfig)
			require.NotNil(t, converted.ToolConfig.IncludeServerSideToolInvocations)
			assert.True(t, *converted.ToolConfig.IncludeServerSideToolInvocations)
		})
	}
}

// A native Gemini client that already decided the flag keeps its own value.
func TestGeminiServerSideToolInvocationsRespectsExplicitClientValue(t *testing.T) {
	t.Parallel()

	tools, err := kitutil.Marshal([]map[string]any{{
		"googleSearch":         map[string]any{},
		"functionDeclarations": []any{map[string]any{"name": "shell"}},
	}})
	require.NoError(t, err)
	disabled := false
	source := &dto.GeminiChatRequest{
		Contents:   []dto.GeminiChatContent{{Role: "user", Parts: []dto.GeminiPart{{Text: "hi"}}}},
		Tools:      tools,
		ToolConfig: &dto.ToolConfig{IncludeServerSideToolInvocations: &disabled},
	}
	_, set, err := ExtractRequest(types.RelayFormatGemini, source)
	require.NoError(t, err)

	target := &dto.GeminiChatRequest{
		Contents: []dto.GeminiChatContent{{Role: "user", Parts: []dto.GeminiPart{{Text: "hi"}}}},
	}
	out, _, err := AttachRequest(types.RelayFormatGemini, target, set, &convmeta.Options{})
	require.NoError(t, err)
	converted, ok := out.(*dto.GeminiChatRequest)
	require.True(t, ok)
	require.NotNil(t, converted.ToolConfig)
	require.NotNil(t, converted.ToolConfig.IncludeServerSideToolInvocations)
	assert.False(t, *converted.ToolConfig.IncludeServerSideToolInvocations)
}
