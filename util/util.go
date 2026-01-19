package util

import (
	"bytes"
	"strings"
	"unicode"
)

// Hytale APIs occasionally reply starting with a Byte Order Mark...
func TrimBOM(body []byte) []byte {
	return bytes.TrimPrefix(body, []byte("\xef\xbb\xbf"))
}

// Convert a string in mixed case to "Capitalized Words Separated By Spaces"
func ToCapitalizedSpacedWords(s string) string {
	if s == "" {
		return ""
	}
	var builder strings.Builder
	for i, r := range s {
		if i == 0 {
			builder.WriteRune(unicode.ToUpper(r))
		} else {
			if unicode.IsSpace(rune(s[i-1])) {
				builder.WriteRune(unicode.ToUpper(r))
			} else if unicode.IsUpper(r) {
				builder.WriteRune(' ')
				builder.WriteRune(unicode.ToUpper(r))
			} else {
				builder.WriteRune(r)
			}
		}
	}
	return builder.String()
}
