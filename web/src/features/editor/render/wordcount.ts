// countWords 与 API 的 content.countWords 保持一致：CJK 字符每字计 1，其余按连续字母数字计词，
// 撇号、连字符和下划线不打断单词。
const cjk = /[\p{Script=Han}\p{Script=Hiragana}\p{Script=Katakana}\p{Script=Hangul}]/u;
const wordChar = /[\p{L}\p{Nd}]/u;
const joiner = /['’\-_]/;

export function countWords(text: string): number {
  let n = 0;
  let inWord = false;
  for (const ch of text) {
    if (cjk.test(ch)) {
      n++;
      inWord = false;
    } else if (wordChar.test(ch)) {
      if (!inWord) {
        n++;
        inWord = true;
      }
    } else if (!joiner.test(ch)) {
      inWord = false;
    }
  }
  return n;
}
