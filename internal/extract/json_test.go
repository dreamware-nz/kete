package extract

import "testing"

func TestExtractJSON(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "bare object",
			in:   `{"score":7,"reasoning":"on-task"}`,
			want: `{"score":7,"reasoning":"on-task"}`,
		},
		{
			name: "fenced with json tag",
			in:   "```json\n{\"score\":7,\"reasoning\":\"on-task\"}\n```",
			want: `{"score":7,"reasoning":"on-task"}`,
		},
		{
			name: "fenced bare",
			in:   "```\n{\"score\":7}\n```",
			want: `{"score":7}`,
		},
		{
			name: "trailing prose after object",
			in:   `{"score":7} — note: tuned for clarity`,
			want: `{"score":7}`,
		},
		{
			name: "leading prose then object",
			in:   `Here is my answer: {"score":7,"reasoning":"x"}`,
			want: `{"score":7,"reasoning":"x"}`,
		},
		{
			name: "string with brace inside",
			in:   `{"score":7,"reasoning":"check }brace inside","x":1}`,
			want: `{"score":7,"reasoning":"check }brace inside","x":1}`,
		},
		{
			name: "fenced with prose around fence",
			in:   "Here you go:\n```json\n{\"a\":1}\n```\nDone.",
			want: `{"a":1}`,
		},
		{
			name: "nested objects",
			in:   `{"summary":{"goal":"x","decisions":[{"choice":"a"}]}}`,
			want: `{"summary":{"goal":"x","decisions":[{"choice":"a"}]}}`,
		},
		{
			name: "no json at all",
			in:   `I cannot answer that question.`,
			want: `I cannot answer that question.`,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := &Response{Content: []ContentBlock{{Type: "text", Text: c.in}}}
			got := r.ExtractJSON()
			if got != c.want {
				t.Errorf("\n in:   %q\n got:  %q\n want: %q", c.in, got, c.want)
			}
		})
	}
}
