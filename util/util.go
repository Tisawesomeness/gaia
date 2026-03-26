package util

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"unicode"

	"github.com/Tisawesomeness/gaia/config"
	"github.com/sony/gobreaker"
)

type webhookMessage struct {
	Content string `json:"content"`
}

// Logs a message to stdout and the Discord webhook, if configured
func DiscordLogf(config *config.Config, httpClient *http.Client, msg string, v ...any) {
	DiscordLog(config, httpClient, fmt.Sprintf(msg, v...))
}

// Logs a message to stdout and the Discord webhook, if configured
func DiscordLog(config *config.Config, httpClient *http.Client, msg string) {
	log.Println(msg)
	if config.LogWebhook == "" {
		return
	}

	body, err := json.Marshal(webhookMessage{msg})
	if err != nil {
		fmt.Printf("Could not log to Discord webhook: %v", err)
		return
	}

	req, err := http.NewRequest("POST", config.LogWebhook, bytes.NewBuffer(body))
	if err != nil {
		fmt.Printf("Could not log to Discord webhook: %v", err)
		return
	}
	req.Header.Add("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		fmt.Printf("Could not log to Discord webhook: %v", err)
		return
	}
	defer resp.Body.Close()
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

// Wraps an angle in degrees to a value between 0 and 359.
func WrapDegrees(degrees int) int {
	if degrees >= 0 {
		return degrees % 360
	}
	return (degrees%360 + 360) % 360
}
