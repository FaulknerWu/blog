package httpx

import (
	"fmt"
	"regexp"
	"unicode/utf8"
)

var slugPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// Validator 累积字段错误，最后由 Err 一次性返回 422。pointer 为 JSON Pointer，如 "#/title"。
type Validator struct {
	errs []FieldError
}

func (v *Validator) Add(pointer, code, detail string) {
	v.errs = append(v.errs, FieldError{Pointer: pointer, Code: code, Detail: detail})
}

func (v *Validator) AddError(e FieldError) {
	v.errs = append(v.errs, e)
}

func (v *Validator) Err() error {
	if len(v.errs) == 0 {
		return nil
	}
	return Validation(v.errs...)
}

func (v *Validator) Length(pointer, s string, minLen, maxLen int) bool {
	n := utf8.RuneCountInString(s)
	switch {
	case n < minLen && minLen == 1:
		v.Add(pointer, "required", "不能为空")
	case n < minLen:
		v.Add(pointer, "too_short", fmt.Sprintf("长度不能少于 %d 个字符", minLen))
	case n > maxLen:
		v.Add(pointer, "too_long", fmt.Sprintf("长度不能超过 %d 个字符", maxLen))
	default:
		return true
	}
	return false
}

func (v *Validator) Slug(pointer, s string, maxLen int) bool {
	switch {
	case s == "":
		v.Add(pointer, "required", "不能为空")
	case len(s) > maxLen:
		v.Add(pointer, "too_long", fmt.Sprintf("长度不能超过 %d 个字符", maxLen))
	case !slugPattern.MatchString(s):
		v.Add(pointer, "invalid_format", "只能包含小写字母、数字和连字符")
	default:
		return true
	}
	return false
}

func (v *Validator) Range(pointer string, n, minVal, maxVal int32) bool {
	if n < minVal || n > maxVal {
		v.Add(pointer, "out_of_range", fmt.Sprintf("取值范围为 %d–%d", minVal, maxVal))
		return false
	}
	return true
}
