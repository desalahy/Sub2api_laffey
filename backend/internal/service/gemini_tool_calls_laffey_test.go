package service

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func laffeyDecodePart(t *testing.T, text string) map[string]any {
	t.Helper()
	var part map[string]any
	d := json.NewDecoder(strings.NewReader(text))
	d.UseNumber()
	if err := d.Decode(&part); err != nil {
		t.Fatal(err)
	}
	return part
}

func TestGeminiToolCallsLaffey_Identity(t *testing.T) {
	for _, withID := range []bool{true, false} {
		s := newGeminiToolPartsLaffey(4096)
		var got []map[string]any
		for i := 0; i < 3; i++ {
			id := ""
			if withID {
				id = fmt.Sprintf(`"id":"call_%d",`, i)
			}
			part := laffeyDecodePart(t, `{"functionCall":{`+id+`"name":"Read","args":{"path":"same"}}}`)
			out, err := s.Push(part)
			if err != nil {
				t.Fatal(err)
			}
			got = append(got, out...)
		}
		if len(got) != 3 {
			t.Fatalf("withID=%v: want 3 calls, got %d", withID, len(got))
		}
		if err := s.Finish(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestGeminiToolCallsLaffey_FragmentsAndOrder(t *testing.T) {
	s := newGeminiToolPartsLaffey(4096)
	inputs := []string{
		`{"functionCall":{"id":"a","name":"Read","args":"{\"path\":\""}}`,
		`{"text":"after a"}`,
		`{"functionCall":{"id":"b","name":"Read","args":{"path":"b"}}}`,
		`{"functionCall":{"id":"a","args":"a\"}"}}`,
	}
	var got []map[string]any
	for i, input := range inputs {
		out, err := s.Push(laffeyDecodePart(t, input))
		if err != nil {
			t.Fatal(err)
		}
		if i < 3 && len(out) != 0 {
			t.Fatalf("delivered later parts before incomplete call: %v", out)
		}
		got = append(got, out...)
	}
	if len(got) != 3 {
		t.Fatalf("want 3 ordered parts, got %v", got)
	}
	if got[0]["functionCall"].(map[string]any)["id"] != "a" || got[1]["text"] != "after a" || got[2]["functionCall"].(map[string]any)["id"] != "b" {
		t.Fatalf("wrong order %v", got)
	}
	if err := s.Finish(); err != nil {
		t.Fatal(err)
	}
}

func TestGeminiToolCallsLaffey_ReplayAndConflict(t *testing.T) {
	s := newGeminiToolPartsLaffey(4096)
	part := `{"functionCall":{"id":"a","name":"Read","args":{"path":"a"}}}`
	if _, err := s.Push(laffeyDecodePart(t, part)); err != nil {
		t.Fatal(err)
	}
	out, err := s.Push(laffeyDecodePart(t, part))
	if err != nil || len(out) != 0 {
		t.Fatalf("replay: %v %v", out, err)
	}
	if _, err := s.Push(laffeyDecodePart(t, `{"functionCall":{"id":"a","name":"Read","args":{"path":"b"}}}`)); err == nil {
		t.Fatal("accepted conflicting call ID")
	}
}

func TestGeminiToolCallsLaffey_Invalid(t *testing.T) {
	for _, input := range []string{
		`{"functionCall":{"name":"Read","args":[]}}`,
		`{"functionCall":{"name":"Read","args":"{}{}"}}`,
		`{"functionCall":{"name":"Read","partialArgs":[{}]}}`,
		`{"functionCall":{"args":{}}}`,
	} {
		s := newGeminiToolPartsLaffey(4096)
		if _, err := s.Push(laffeyDecodePart(t, input)); err == nil {
			t.Fatalf("accepted %s", input)
		}
	}
	s := newGeminiToolPartsLaffey(4096)
	out, err := s.Push(laffeyDecodePart(t, `{"functionCall":{"id":"a","name":"Read"}}`))
	if err != nil || len(out) != 0 {
		t.Fatalf("missing args emitted as empty: %v %v", out, err)
	}
	if err := s.Finish(); err == nil {
		t.Fatal("accepted incomplete EOF")
	}
	s = newGeminiToolPartsLaffey(16)
	if _, err := s.Push(laffeyDecodePart(t, `{"functionCall":{"name":"Read","args":{}}}`)); err == nil {
		t.Fatal("ignored byte budget")
	}
}

func TestGeminiToolCallsLaffey_CumulativeAndEscapes(t *testing.T) {
	s := newGeminiToolPartsLaffey(8192)
	for _, args := range []string{`{"text":"`, `{"text":"}{\"你好`, `{"text":"}{\"你好","number":9007199254740993}`} {
		argBytes, _ := json.Marshal(args)
		out, err := s.Push(laffeyDecodePart(t, `{"functionCall":{"id":"a","name":"Read","args":`+string(argBytes)+`}}`))
		if err != nil {
			t.Fatal(err)
		}
		if len(out) > 0 {
			obj := out[0]["functionCall"].(map[string]any)["args"].(map[string]any)
			if obj["number"] != json.Number("9007199254740993") || obj["text"] != "}{\"你好" {
				t.Fatalf("corrupt args: %v", obj)
			}
		}
	}
	if err := s.Finish(); err != nil {
		t.Fatal(err)
	}
}

func BenchmarkGeminiToolCallsLaffey(b *testing.B) {
	for _, size := range []int{4096, 16384, 65536} {
		b.Run(fmt.Sprint(size), func(b *testing.B) {
			args := `{"text":"` + strings.Repeat("x", size) + `"}`
			b.ReportAllocs()
			b.SetBytes(int64(len(args)))
			for n := 0; n < b.N; n++ {
				s := newGeminiToolPartsLaffey(int64(size * 4))
				for i := 0; i < len(args); i += 128 {
					end := i + 128
					if end > len(args) {
						end = len(args)
					}
					_, err := s.Push(map[string]any{"functionCall": map[string]any{"id": "a", "name": "Read", "args": args[i:end]}})
					if err != nil {
						b.Fatal(err)
					}
				}
				if err := s.Finish(); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
