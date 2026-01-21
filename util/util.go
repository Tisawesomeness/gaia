package util

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"unicode"

	"github.com/sony/gobreaker"
)

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

func Execute[T any](breaker *gobreaker.CircuitBreaker, req func() (T, error)) (T, error) {
	var result T
	_, err := breaker.Execute(func() (any, error) {
		innerResult, innerErr := req()
		result = innerResult
		return nil, innerErr
	})
	return result, err
}

// Returns an error with HTTP code, headers, and body attached
func NewBadResponseError(description string, resp *http.Response) error {
	finalURL := resp.Request.URL.String()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("%s: %s %s returned HTTP %d, %v", description, resp.Request.Method, finalURL, resp.StatusCode, err)
	} else {
		bodyStr := string(body)
		return fmt.Errorf("%s: %s %s returned HTTP %d:\n%s\n%s\n", description, resp.Request.Method, finalURL, resp.StatusCode, resp.Header, bodyStr[:min(50, len(bodyStr))])
	}
}
