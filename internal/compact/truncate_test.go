package compact

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestTruncateLargeBody_NoOpWhenSmall(t *testing.T) {
	body := []byte(`{"model":"x","messages":[{"role":"user","content":"hi"}]}`)
	out, did, err := TruncateLargeBody(body, 5)
	if err != nil {
		t.Fatal(err)
	}
	if did {
		t.Fatalf("did=true on small body")
	}
	if string(out) != string(body) {
		t.Fatalf("body mutated when no-op")
	}
}

func TestTruncateLargeBody_KeepsLastK(t *testing.T) {
	var sb strings.Builder
	sb.WriteString(`{"model":"x","system":"sys","messages":[`)
	for i := 0; i < 50; i++ {
		if i > 0 {
			sb.WriteByte(',')
		}
		sb.WriteString(`{"role":"`)
		if i%2 == 0 {
			sb.WriteString(`user`)
		} else {
			sb.WriteString(`assistant`)
		}
		sb.WriteString(`","content":"msg-`)
		sb.WriteString(itoa(i))
		sb.WriteString(`"}`)
	}
	sb.WriteString(`]}`)
	out, did, err := TruncateLargeBody([]byte(sb.String()), 10)
	if err != nil {
		t.Fatal(err)
	}
	if !did {
		t.Fatalf("did=false; expected truncation")
	}
	var probe struct {
		System   string `json:"system"`
		Messages []struct {
			Role    string `json:"role"`
			Content any    `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(out, &probe); err != nil {
		t.Fatal(err)
	}
	if probe.System != "sys" {
		t.Errorf("system lost: %q", probe.System)
	}
	if got, want := len(probe.Messages), 1+10; got != want {
		t.Fatalf("len(messages)=%d, want %d (1 marker + keepLast)", got, want)
	}
	first := probe.Messages[0]
	blocks, ok := first.Content.([]any)
	if !ok || len(blocks) == 0 {
		t.Fatalf("marker content wrong shape: %T", first.Content)
	}
	blkMap := blocks[0].(map[string]any)
	text := blkMap["text"].(string)
	if !strings.Contains(text, "kete:truncation") {
		t.Errorf("marker missing tag: %q", text)
	}
	last := probe.Messages[len(probe.Messages)-1].Content.(string)
	if last != "msg-49" {
		t.Errorf("tail mangled; last=%q want msg-49", last)
	}
}

// itoa avoids strconv to keep the test self-contained and obvious.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
