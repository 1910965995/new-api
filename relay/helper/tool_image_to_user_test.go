package helper

import (
	"testing"

	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func toolImageRequest(messages ...dto.Message) *dto.GeneralOpenAIRequest {
	return &dto.GeneralOpenAIRequest{Messages: messages}
}

func TestMoveToolImagesToUserMessagesMovesImageAfterToolRun(t *testing.T) {
	request := toolImageRequest(
		dto.Message{Role: "user", Content: "read the image"},
		dto.Message{Role: "tool", ToolCallId: "call_1", Content: []any{
			map[string]any{"type": "text", "text": "file content:"},
			map[string]any{"type": "image_url", "image_url": map[string]any{"url": "data:image/png;base64,AAA"}},
		}},
	)

	MoveToolImagesToUserMessages(request)

	require.Len(t, request.Messages, 3)
	assert.Equal(t, "tool", request.Messages[1].Role)
	blocks := request.Messages[1].ParseContent()
	require.Len(t, blocks, 1)
	assert.Equal(t, dto.ContentTypeText, blocks[0].Type)
	assert.Equal(t, "file content:", blocks[0].Text)

	assert.Equal(t, "user", request.Messages[2].Role)
	moved := request.Messages[2].ParseContent()
	require.Len(t, moved, 2)
	assert.Equal(t, dto.ContentTypeText, moved[0].Type)
	assert.Contains(t, moved[0].Text, "tool results")
	assert.Equal(t, dto.ContentTypeImageURL, moved[1].Type)
	assert.Equal(t, "data:image/png;base64,AAA", moved[1].GetImageMedia().Url)
}

func TestMoveToolImagesToUserMessagesReplacesImageOnlyToolContent(t *testing.T) {
	request := toolImageRequest(
		dto.Message{Role: "tool", ToolCallId: "call_1", Content: []any{
			map[string]any{"type": "image_url", "image_url": map[string]any{"url": "data:image/png;base64,BBB"}},
		}},
		dto.Message{Role: "assistant", Content: "done"},
	)

	MoveToolImagesToUserMessages(request)

	require.Len(t, request.Messages, 3)
	assert.Equal(t, "tool", request.Messages[0].Role)
	assert.Equal(t, "[image content moved to the next user message]", request.Messages[0].Content)
	assert.Equal(t, "user", request.Messages[1].Role)
	assert.Equal(t, "assistant", request.Messages[2].Role)
}

func TestMoveToolImagesToUserMessagesLeavesTextOnlyRequestsUntouched(t *testing.T) {
	request := toolImageRequest(
		dto.Message{Role: "user", Content: []any{
			map[string]any{"type": "text", "text": "hi"},
			map[string]any{"type": "image_url", "image_url": map[string]any{"url": "data:image/png;base64,CCC"}},
		}},
		dto.Message{Role: "tool", ToolCallId: "call_1", Content: "plain text output"},
		dto.Message{Role: "tool", ToolCallId: "call_1", Content: []any{
			map[string]any{"type": "text", "text": "array text output"},
		}},
	)

	MoveToolImagesToUserMessages(request)

	require.Len(t, request.Messages, 3)
	assert.Equal(t, "plain text output", request.Messages[1].Content)
	assert.Equal(t, []any{map[string]any{"type": "text", "text": "array text output"}}, request.Messages[2].Content)
}
