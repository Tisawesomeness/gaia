package auth

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/Tisawesomeness/gaia/config"
	"github.com/Tisawesomeness/gaia/util"
	"github.com/pquerna/otp/totp"
)

const KratosSessionCookie = "ory_kratos_session"

func extractFlowID(url string) (string, error) {
	re := regexp.MustCompile(`flow=([a-f0-9-]+)`)
	matches := re.FindStringSubmatch(url)
	if len(matches) < 2 {
		return "", fmt.Errorf("could not extract flow ID from url %s", url)
	}
	return matches[1], nil
}

func getCSRFToken(config *config.Config, httpClient *http.Client) (string, error) {
	cookieURL, err := url.Parse(config.Kratos.AccountsBackend)
	if err != nil {
		return "", err
	}

	cookies := httpClient.Jar.Cookies(cookieURL)
	for _, cookie := range cookies {
		if strings.HasPrefix(cookie.Name, "csrf_token") {
			return cookie.Value, nil
		}
	}
	return "", errors.New("could not find CSRF token cookie")
}

// Initiates the Kratos login flow with username, password, and 2FA secret.
// Cookies are stored in the provided httpClient
func KratosLogin(config *config.Config, httpClient *http.Client) error {
	if httpClient.Jar == nil {
		return errors.New("httpClient must have a cookiejar")
	}
	username := config.Credentials.HytaleEmail
	password := config.Credentials.HytalePassword

	// Step 1: Create browser login flow
	resp, err := httpClient.Get(config.Kratos.AccountsBackend + "/self-service/login/browser")
	if err != nil {
		return fmt.Errorf("failed to fetch login UI: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return util.NewBadResponseError("failed to fetch login", resp)
	}

	// Extract flow ID from URL redirect
	finalURL := resp.Request.URL.String()
	flowID, err := extractFlowID(finalURL)
	if err != nil {
		return err
	}

	csrfToken, err := getCSRFToken(config, httpClient)
	if err != nil {
		return err
	}

	// Step 2: Submit login flow with method=password
	formData := url.Values{}
	formData.Set("csrf_token", csrfToken)
	formData.Set("identifier", username)
	formData.Set("password", password)
	formData.Set("method", "password")

	loginURL := fmt.Sprintf("%s/self-service/login?flow=%s", config.Kratos.AccountsBackend, flowID)
	resp, err = httpClient.Post(loginURL, "application/x-www-form-urlencoded", strings.NewReader(formData.Encode()))
	if err != nil {
		return fmt.Errorf("failed to submit login form: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return util.NewBadResponseError("failed to submit login", resp)
	}

	// Expect a redirect to another flow ID for 2FA
	finalURL = resp.Request.URL.String()
	flowID, err = extractFlowID(finalURL)
	if err != nil {
		return fmt.Errorf("failed to extract 2FA flow ID, is 2FA enabled?: %v", err)
	}

	// Step 3: 2FA
	err = submit2FA(flowID, config, httpClient)
	if err != nil {
		return fmt.Errorf("2FA verification failed: %v", err)
	}
	return nil
}

func submit2FA(flowID string, config *config.Config, httpClient *http.Client) error {
	csrfToken, err := getCSRFToken(config, httpClient)
	if err != nil {
		return fmt.Errorf("could not find CSRF token cookie for 2FA: %v", err)
	}

	totpCode, err := totp.GenerateCode(config.Credentials.Hytale2FASecret, time.Now())
	if err != nil {
		return fmt.Errorf("failed to generate TOTP code: %v", err)
	}

	formData := url.Values{}
	formData.Set("csrf_token", csrfToken)
	formData.Set("totp_code", totpCode)
	formData.Set("method", "totp")

	loginURL := fmt.Sprintf("%s/self-service/login?flow=%s", config.Kratos.AccountsBackend, flowID)
	resp, err := httpClient.Post(loginURL, "application/x-www-form-urlencoded", strings.NewReader(formData.Encode()))
	if err != nil {
		return fmt.Errorf("failed to submit 2FA form: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return util.NewBadResponseError("failed to submit 2FA", resp)
	}

	// A successful login redirects to the account settings page
	finalURL := resp.Request.URL
	if !strings.Contains(finalURL.String(), "/settings") {
		return fmt.Errorf("Bad login, ended up at %s", finalURL)
	}

	for _, cookie := range httpClient.Jar.Cookies(finalURL) {
		if cookie.Name == KratosSessionCookie {
			return nil
		}
	}
	return errors.New("Missing session cookie")
}
