package content

import "testing"

func TestCountWords(t *testing.T) {
	cases := map[string]int{
		"":                         0,
		"写时渲染":                     4,
		"hello world":              2,
		"用 pg_trgm 做中文检索":          7,
		"don't stop-me now, 2026年": 5,
		"  \n\t ":                  0,
	}
	for in, want := range cases {
		if got := countWords(in); got != want {
			t.Errorf("countWords(%q) = %d, want %d", in, got, want)
		}
	}
}
