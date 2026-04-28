package content

import (
	"unicode"
)

// countWords 统计中英混排文本的字数：CJK 字符每字计 1，其余按连续字母数字计词，
// 撇号、连字符和下划线不打断单词（如 don't、e-mail 各计 1）。
func countWords(s string) int {
	n := 0
	inWord := false
	for _, r := range s {
		switch {
		case isCJK(r):
			n++
			inWord = false
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			if !inWord {
				n++
				inWord = true
			}
		case r == '\'' || r == '’' || r == '-' || r == '_':
		default:
			inWord = false
		}
	}
	return n
}

func isCJK(r rune) bool {
	return unicode.Is(unicode.Han, r) ||
		unicode.Is(unicode.Hiragana, r) ||
		unicode.Is(unicode.Katakana, r) ||
		unicode.Is(unicode.Hangul, r)
}
