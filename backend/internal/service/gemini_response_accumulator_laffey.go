package service

import (
	"crypto/sha256"
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
	candidates  map[int]*geminiResponseAccumulatorLaffey
	order       []int
	images      map[[32]byte]int
	sawDone     bool
	sawCall     bool
}

func newGeminiResponseAccumulatorLaffey(limit int64) *geminiResponseAccumulatorLaffey {
	return &geminiResponseAccumulatorLaffey{response: make(map[string]any), candidate: make(map[string]any), tools: newGeminiToolPartsLaffey(limit), limit: limit, candidates: make(map[int]*geminiResponseAccumulatorLaffey), images: make(map[[32]byte]int)}
}

func (a *geminiResponseAccumulatorLaffey) Add(response map[string]any) error {
	raw, err := json.Marshal(response)
	if err != nil {
		return err
	}
	if int64(len(raw)) > a.limit-a.bytes {
		return errors.New("gemini response exceeds accumulation byte limit")
	}
	a.bytes += int64(len(raw))
	for key, value := range response {
		if key != "candidates" {
			a.response[key] = value
		}
	}
	if candidates, ok := response["candidates"].([]any); ok {
		for position, item := range candidates {
			candidate, ok := item.(map[string]any)
			if !ok {
				continue
			}
			index := position
			if value, ok := asInt(candidate["index"]); ok {
				index = value
			}
			child := a.candidates[index]
			if child == nil {
				child = newGeminiResponseAccumulatorLaffey(a.limit)
				a.candidates[index] = child
				a.order = append(a.order, index)
			}
			if err := child.addCandidate(candidate); err != nil {
				return err
			}
		}
	}
	return nil
}

func (a *geminiResponseAccumulatorLaffey) addCandidate(candidate map[string]any) error {
	for key, value := range candidate {
		if key != "content" {
			a.candidate[key] = value
		}
	}
	frame := map[string]any{"candidates": []any{candidate}}
	var parts []map[string]any
	frameImages := make(map[[32]byte]int)
	for _, part := range extractGeminiParts(frame) {
		_, call := part["functionCall"]
		if call {
			a.sawCall = true
		}
		if geminiImagePartLaffey(part) {
			raw, err := json.Marshal(part)
			if err != nil {
				return err
			}
			hash := sha256.Sum256(raw)
			frameImages[hash]++
			// Image payloads may be cumulative snapshots. Retain the largest
			// multiplicity within any frame, matching native image accounting.
			if frameImages[hash] <= a.images[hash] {
				continue
			}
			a.images[hash] = frameImages[hash]
		}
		ready, err := a.tools.Push(part)
		if err != nil {
			return err
		}
		parts = append(parts, ready...)
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
	var candidates []any
	for _, index := range a.order {
		child := a.candidates[index]
		if err := child.tools.Finish(); err != nil {
			return nil, err
		}
		_, upstreamError := a.response["error"]
		if child.sawCall && child.candidate["finishReason"] == nil && !a.sawDone && !upstreamError {
			return nil, errors.New("gemini tool response ended without a terminal event")
		}
		child.flushText()
		if len(child.parts) > 0 {
			child.candidate["content"] = map[string]any{"role": "model", "parts": child.parts}
		}
		candidates = append(candidates, child.candidate)
	}
	if len(candidates) > 0 {
		a.response["candidates"] = candidates
	}
	return a.response, nil
}

func geminiImagePartLaffey(part map[string]any) bool {
	for _, key := range []string{"inlineData", "inline_data", "fileData", "file_data"} {
		if data, ok := part[key].(map[string]any); ok {
			mime, _ := data["mimeType"].(string)
			if mime == "" {
				mime, _ = data["mime_type"].(string)
			}
			if isGeminiInlineImageMIMEType(mime) {
				return true
			}
		}
	}
	return false
}
