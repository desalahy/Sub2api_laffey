package service

import (
	"encoding/json"
	"errors"
)

// Keep final metadata independently from accumulated content: a final usage or
// finish-only chunk must not discard earlier tool calls, signatures or images.
type geminiResponseAccumulatorLaffey struct {
	response    map[string]any
	candidate   map[string]any
	parts       []any
	tools       *geminiToolPartsLaffey
	text        geminiTextDelta
	textPart    map[string]any
	textThought bool
	limit       int64
	bytes       int64
}

func newGeminiResponseAccumulatorLaffey(limit int64) *geminiResponseAccumulatorLaffey {
	return &geminiResponseAccumulatorLaffey{response: make(map[string]any), candidate: make(map[string]any), tools: newGeminiToolPartsLaffey(limit), limit: limit}
}

func (a *geminiResponseAccumulatorLaffey) Add(response map[string]any) error {
	raw, err := json.Marshal(response)
	if err != nil {
		return err
	}
	if int64(len(raw)) > a.limit-a.bytes {
		return errors.New("Gemini response exceeds accumulation byte limit")
	}
	a.bytes += int64(len(raw))
	for key, value := range response {
		if key != "candidates" {
			a.response[key] = value
		}
	}
	if candidates, ok := response["candidates"].([]any); ok && len(candidates) > 0 {
		if candidate, ok := candidates[0].(map[string]any); ok {
			for key, value := range candidate {
				if key != "content" {
					a.candidate[key] = value
				}
			}
		}
	}
	parts, err := a.tools.Frame(response)
	if err != nil {
		return err
	}
	for _, part := range parts {
		text, isText := part["text"].(string)
		_, isCall := part["functionCall"]
		_, isImage := part["inlineData"]
		if isText && !isCall && !isImage {
			thought := isGeminiThoughtPart(part)
			// Signed parts are atomic. Never move a signature onto a different
			// text part or concatenate two independently signed parts.
			_, signed := part["thoughtSignature"]
			if signed {
				a.flushText()
				a.parts = append(a.parts, part)
				continue
			}
			if a.textPart == nil || a.textThought != thought {
				a.flushText()
				a.textPart = part
				a.textThought = thought
			}
			a.text.next(text)
		} else {
			a.flushText()
			a.parts = append(a.parts, part)
		}
	}
	return nil
}

func (a *geminiResponseAccumulatorLaffey) flushText() {
	if a.textPart == nil {
		return
	}
	a.textPart["text"] = a.text.seen.String()
	a.parts = append(a.parts, a.textPart)
	a.textPart = nil
	a.text.reset()
}

func (a *geminiResponseAccumulatorLaffey) Finish() (map[string]any, error) {
	if err := a.tools.Finish(); err != nil {
		return nil, err
	}
	a.flushText()
	if len(a.parts) > 0 {
		a.candidate["content"] = map[string]any{"role": "model", "parts": a.parts}
	}
	if len(a.candidate) > 0 {
		a.response["candidates"] = []any{a.candidate}
	}
	return a.response, nil
}
