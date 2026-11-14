package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

// geminiToolPartsLaffey preserves call identity and original part order. A
// complete functionCall is one call, even when its name and args equal another
// call's. Only explicitly identified (or unambiguous unfinished) calls accept
// argument fragments. The queue also holds text arriving behind an unfinished
// call, so the sequential Anthropic bridge never observes reordered parts.
type geminiToolPartsLaffey struct {
	limit     int64
	bytes     int64
	byID      map[string]*geminiToolPartLaffey
	anonymous *geminiToolPartLaffey
	queue     []*geminiToolPartLaffey
}

type geminiToolPartLaffey struct {
	part      map[string]any
	name      string
	complete  bool
	arguments strings.Builder
	canonical string
	scan      geminiJSONScanLaffey
}

func newGeminiToolPartsLaffey(maxBytes int64) *geminiToolPartsLaffey {
	return &geminiToolPartsLaffey{limit: maxBytes, byID: make(map[string]*geminiToolPartLaffey)}
}

// Push owns a deep copy. The cumulative input budget bounds both queued parts
// and the identity records retained to detect replays; many short fragments
// cannot evade the limit. JSON numbers must remain lossless at the reader too.
func (s *geminiToolPartsLaffey) Push(part map[string]any) ([]map[string]any, error) {
	raw, err := json.Marshal(part)
	if err != nil {
		return nil, err
	}
	if int64(len(raw)) > s.limit-s.bytes {
		return nil, errors.New("gemini tool stream exceeds response byte limit")
	}
	s.bytes += int64(len(raw))
	owned, err := decodeGeminiObjectLaffey(raw)
	if err != nil {
		return nil, err
	}
	value, hasCall := owned["functionCall"]
	if !hasCall {
		s.queue = append(s.queue, &geminiToolPartLaffey{part: owned, complete: true})
		return s.drain(), nil
	}
	fc, ok := value.(map[string]any)
	if !ok {
		return nil, errors.New("invalid Gemini functionCall")
	}
	if _, ok := fc["partialArgs"]; ok {
		return nil, errors.New("gemini partialArgs streaming is not supported")
	}
	if continued, _ := fc["willContinue"].(bool); continued {
		return nil, errors.New("gemini willContinue streaming is not supported")
	}
	id, _ := fc["id"].(string)
	name, _ := fc["name"].(string)
	entry := s.byID[id]
	if id == "" {
		entry = s.anonymous
		// A complete ID-less part is a new call, not a fragment of the
		// unfinished anonymous call (which keeps its position in the queue).
		if _, completeObject := fc["args"].(map[string]any); completeObject {
			entry = nil
		} else if text, ok := fc["args"].(string); ok && json.Valid([]byte(text)) {
			entry = nil
		}
	}
	if entry != nil && name != "" && entry.name != name {
		return nil, errors.New("conflicting Gemini tool name for call ID")
	}
	if entry == nil {
		if strings.TrimSpace(name) == "" {
			return nil, errors.New("gemini functionCall is missing name")
		}
		entry = &geminiToolPartLaffey{part: owned, name: name}
		s.queue = append(s.queue, entry)
		if id != "" {
			s.byID[id] = entry
		} else if s.anonymous == nil {
			s.anonymous = entry
		}
	}
	if sig, ok := owned["thoughtSignature"]; ok {
		entry.part["thoughtSignature"] = sig
	}
	args, hasArgs := fc["args"]
	if hasArgs {
		if err := entry.appendArgs(args); err != nil {
			return nil, err
		}
	}
	if entry.complete && s.anonymous == entry {
		s.anonymous = nil
	}
	return s.drain(), nil
}

func (s *geminiToolPartsLaffey) drain() []map[string]any {
	var out []map[string]any
	n := 0
	for n < len(s.queue) && s.queue[n].complete {
		out = append(out, s.queue[n].part)
		s.queue[n] = nil
		n++
	}
	s.queue = s.queue[n:]
	return out
}

func (s *geminiToolPartsLaffey) Finish() error {
	if len(s.queue) != 0 {
		return errors.New("incomplete Gemini tool arguments at end of stream")
	}
	return nil
}

func (s *geminiToolPartsLaffey) Frame(response map[string]any) ([]map[string]any, error) {
	var out []map[string]any
	for _, part := range extractGeminiParts(response) {
		ready, err := s.Push(part)
		if err != nil {
			return nil, err
		}
		out = append(out, ready...)
	}
	return out, nil
}

func (p *geminiToolPartLaffey) appendArgs(args any) error {
	var text string
	switch value := args.(type) {
	case map[string]any:
		raw, err := json.Marshal(value)
		if err != nil {
			return err
		}
		text = string(raw)
	case string:
		text = strings.TrimSuffix(value, "\x00")
	default:
		return errors.New("gemini tool arguments must be a JSON object")
	}
	if p.complete {
		obj, err := decodeGeminiObjectLaffey([]byte(text))
		if err != nil {
			return errors.New("conflicting Gemini tool arguments for completed call ID")
		}
		raw, _ := json.Marshal(obj)
		if string(raw) != p.canonical {
			return errors.New("conflicting Gemini tool arguments for completed call ID")
		}
		return nil
	}
	seen := p.arguments.String()
	delta := text
	if strings.HasPrefix(text, seen) {
		delta = text[len(seen):]
	}
	if err := p.scan.Push(delta); err != nil {
		return err
	}
	_, _ = p.arguments.WriteString(delta)
	if !p.scan.done {
		return nil
	}
	obj, err := decodeGeminiObjectLaffey([]byte(p.arguments.String()))
	if err != nil {
		return fmt.Errorf("invalid Gemini tool arguments: %w", err)
	}
	raw, _ := json.Marshal(obj)
	p.canonical = string(raw)
	call, ok := p.part["functionCall"].(map[string]any)
	if !ok {
		return errors.New("invalid normalized Gemini functionCall")
	}
	call["args"] = obj
	p.complete = true
	p.arguments.Reset()
	return nil
}

// A linear structural scan finds the end of a fragmented object. The JSON
// decoder validates the grammar exactly once when the object is complete.
type geminiJSONScanLaffey struct {
	depth   int
	quoted  bool
	escaped bool
	started bool
	done    bool
}

func (s *geminiJSONScanLaffey) Push(text string) error {
	for i := 0; i < len(text); i++ {
		c := text[i]
		if s.quoted {
			if s.escaped {
				s.escaped = false
			} else if c == '\\' {
				s.escaped = true
			} else if c == '"' {
				s.quoted = false
			}
			continue
		}
		if c == ' ' || c == '\n' || c == '\r' || c == '\t' {
			continue
		}
		if s.done {
			return errors.New("multiple JSON values in Gemini tool arguments")
		}
		if !s.started {
			if c != '{' {
				return errors.New("gemini tool arguments must start with an object")
			}
			s.started = true
		}
		switch c {
		case '"':
			s.quoted = true
		case '{', '[':
			s.depth++
		case '}', ']':
			s.depth--
			if s.depth < 0 {
				return errors.New("unbalanced Gemini tool arguments")
			}
			if s.depth == 0 {
				s.done = true
			}
		}
	}
	return nil
}

func decodeGeminiObjectLaffey(raw []byte) (map[string]any, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var object map[string]any
	if err := decoder.Decode(&object); err != nil {
		return nil, err
	}
	if object == nil {
		return nil, errors.New("expected JSON object")
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return nil, errors.New("expected one JSON object")
	}
	return object, nil
}
