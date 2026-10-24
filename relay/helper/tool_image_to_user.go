package helper

import (
	"github.com/QuantumNous/new-api/relaykit/dto"
)

const (
	toolImageMovedNotice      = "[images moved from tool results]"
	toolImageMovedPlaceholder = "[image content moved to the next user message]"
)

// MoveToolImagesToUserMessages rewrites chat history so image parts carried in
// tool messages ride in a user message appended after the tool run instead.
// Some OpenAI-compatible upstreams only accept text tool outputs ("Only text
// Chat tool output is supported"), so a client that returns images as tool
// results always fails; moving the images keeps the same conversation working
// while the tool messages keep their text.
func MoveToolImagesToUserMessages(request *dto.GeneralOpenAIRequest) {
	moved := false
	rebuilt := make([]dto.Message, 0, len(request.Messages))
	var pendingImages []dto.MediaContent

	flushImages := func() {
		if len(pendingImages) == 0 {
			return
		}
		content := make([]dto.MediaContent, 0, len(pendingImages)+1)
		content = append(content, dto.MediaContent{Type: dto.ContentTypeText, Text: toolImageMovedNotice})
		content = append(content, pendingImages...)
		rebuilt = append(rebuilt, dto.Message{Role: "user", Content: content})
		pendingImages = nil
	}

	for _, message := range request.Messages {
		if message.Role != "tool" {
			flushImages()
			rebuilt = append(rebuilt, message)
			continue
		}
		blocks := message.ParseContent()
		hasImage := false
		for _, block := range blocks {
			if block.Type == dto.ContentTypeImageURL {
				hasImage = true
				break
			}
		}
		if !hasImage {
			rebuilt = append(rebuilt, message)
			continue
		}
		kept := make([]dto.MediaContent, 0, len(blocks))
		for _, block := range blocks {
			if block.Type == dto.ContentTypeImageURL {
				pendingImages = append(pendingImages, block)
				continue
			}
			kept = append(kept, block)
		}
		if len(kept) == 0 {
			message.SetStringContent(toolImageMovedPlaceholder)
		} else {
			message.SetMediaContent(kept)
		}
		moved = true
		rebuilt = append(rebuilt, message)
	}
	flushImages()

	if moved {
		request.Messages = rebuilt
	}
}
