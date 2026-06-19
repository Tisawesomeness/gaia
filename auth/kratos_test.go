package auth

import (
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"testing"
	"time"

	"github.com/Tisawesomeness/gaia/config"
	"github.com/Tisawesomeness/gaia/testutil"
	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
)

// passwordURL: the URL to redirect to when method=password, fails with 400 instead if empty
// totpURL: the URL to redirect to when method=totp, fails with 400 instead if empty
func loginFlowResponder(passwordURL string, totpURL string) httpmock.Responder {
	return func(req *http.Request) (*http.Response, error) {
		req.ParseForm()
		method := req.Form.Get("method")
		switch method {
		case "password":
			if passwordURL != "" {
				// Password step - redirect to 2FA challenge
				resp := httpmock.NewStringResponse(302, "")
				resp.Header.Set("Location", passwordURL)
				return resp, nil
			} else {
				return httpmock.NewStringResponse(400, "Login failed"), nil
			}
		case "totp":
			if totpURL != "" {
				// 2FA step - redirect to settings
				resp := httpmock.NewStringResponse(302, "")
				resp.Header.Set("Location", totpURL)
				return resp, nil
			} else {
				return httpmock.NewStringResponse(400, "Login failed"), nil
			}
		default:
			return httpmock.NewStringResponse(400, "Invalid method"), nil
		}
	}
}

func setupMockClient(t *testing.T) *http.Client {
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	mockClient := &http.Client{
		Timeout: 10 * time.Second,
		Jar:     jar,
	}
	httpmock.ActivateNonDefault(mockClient)
	return mockClient
}

func assertKratosSession(t *testing.T, http *http.Client) {
	url, _ := url.Parse("http://mock.kratos")
	for _, cookie := range http.Jar.Cookies(url) {
		if cookie.Name == KratosSessionCookie {
			return
		}
	}
	t.Error("Could not find " + KratosSessionCookie + " cookie")
}

func TestKratos(t *testing.T) {
	config := &config.Config{
		Credentials: config.CredentialsConfig{
			HytaleEmail:     "test@example.com",
			HytalePassword:  "test123",
			Hytale2FASecret: "T7HK43UPHIYJGWGDYFGBQKGHKUT47G4Z",
		},
		Kratos: config.KratosConfig{
			AccountsBackend: "http://mock.kratos",
		},
	}

	t.Run("browser login initiates flow and redirects to 2FA", func(t *testing.T) {
		mockClient := setupMockClient(t)

		// Step 1a: Redirect to login page
		httpmock.RegisterResponder(
			"GET",
			"http://mock.kratos/self-service/login/browser",
			testutil.WithRequest(httpmock.NewStringResponder(302, "").HeaderSet(http.Header{
				"Location": []string{"http://mock.kratos/example-login-page?flow=abc123-def456"},
			})),
		)

		// Step 1b: Return login page with CSRF token
		httpmock.RegisterResponder(
			"GET",
			"http://mock.kratos/example-login-page",
			testutil.WithRequest(httpmock.NewStringResponder(200, "").HeaderSet(http.Header{
				"Set-Cookie": []string{"csrf_token=123456"},
			})),
		)

		// Step 2-3: Form submission (password and 2FA)
		httpmock.RegisterResponder(
			"POST",
			"http://mock.kratos/self-service/login",
			testutil.WithRequest(loginFlowResponder("http://mock.kratos/example-login-page?flow=2fapage", "http://mock.kratos/settings/public/uid/testuser")),
		)

		// Finish: Return settings page with login cookie
		httpmock.RegisterResponder(
			"GET",
			"http://mock.kratos/settings/public/uid/testuser",
			testutil.WithRequest(httpmock.NewStringResponder(200, "<html><body>you did it</body></html>").HeaderSet(http.Header{
				"Set-Cookie": []string{"ory_kratos_session=eyJhb; Path=/"},
			})),
		)

		err := KratosLogin(config, mockClient)
		assert.NoError(t, err)
		assertKratosSession(t, mockClient)
	})

	t.Run("fails when httpClient has no cookiejar", func(t *testing.T) {
		client := &http.Client{}
		err := KratosLogin(config, client)
		assert.ErrorContains(t, err, "httpClient must have a cookiejar")
	})

	t.Run("fails when getting login page returns network error", func(t *testing.T) {
		mockClient := setupMockClient(t)

		httpmock.RegisterResponder("GET", "http://mock.kratos/self-service/login/browser",
			httpmock.NewStringResponder(500, "Internal Server Error"))

		err := KratosLogin(config, mockClient)
		assert.ErrorContains(t, err, "failed to fetch login")
	})

	t.Run("fails when login page returns non-200 status", func(t *testing.T) {
		mockClient := setupMockClient(t)

		httpmock.RegisterResponder("GET", "http://mock.kratos/self-service/login/browser",
			httpmock.NewStringResponder(503, "Service Unavailable"))

		err := KratosLogin(config, mockClient)
		assert.ErrorContains(t, err, "failed to fetch login")
	})

	t.Run("fails when flow ID cannot be extracted from redirect", func(t *testing.T) {
		mockClient := setupMockClient(t)

		httpmock.RegisterResponder(
			"GET",
			"http://mock.kratos/self-service/login/browser",
			testutil.WithRequest(httpmock.NewStringResponder(302, "").HeaderSet(http.Header{
				"Location": []string{"http://mock.kratos/example-login-page"}, // No flow= param
			})),
		)

		httpmock.RegisterResponder(
			"GET",
			"http://mock.kratos/example-login-page",
			httpmock.NewStringResponder(200, "").HeaderSet(http.Header{
				"Set-Cookie": []string{"csrf_token=123456"},
			}),
		)

		err := KratosLogin(config, mockClient)
		assert.ErrorContains(t, err, "could not extract flow ID")
	})

	t.Run("fails when CSRF token cookie is missing", func(t *testing.T) {
		mockClient := setupMockClient(t)

		// Browser redirect
		httpmock.RegisterResponder(
			"GET",
			"http://mock.kratos/self-service/login/browser",
			testutil.WithRequest(httpmock.NewStringResponder(302, "").HeaderSet(http.Header{
				"Location": []string{"http://mock.kratos/example-login-page?flow=abc123-def456"},
			})),
		)

		//  Login page WITHOUT Set-Cookie
		httpmock.RegisterResponder(
			"GET",
			"http://mock.kratos/example-login-page",
			testutil.WithRequest(func(req *http.Request) (*http.Response, error) {
				return httpmock.NewStringResponse(200, "<html>Login</html>"), nil
			}),
		)

		// POST will fail because no CSRF cookie will be in jar
		err := KratosLogin(config, mockClient)
		assert.ErrorContains(t, err, "could not find CSRF token cookie")
	})

	t.Run("fails when password form returns 400", func(t *testing.T) {
		mockClient := setupMockClient(t)

		httpmock.RegisterResponder(
			"GET",
			"http://mock.kratos/self-service/login/browser",
			testutil.WithRequest(httpmock.NewStringResponder(302, "").HeaderSet(http.Header{
				"Location": []string{"http://mock.kratos/example-login-page?flow=abc123-def456"},
			})),
		)

		httpmock.RegisterResponder(
			"GET",
			"http://mock.kratos/example-login-page",
			httpmock.NewStringResponder(200, "").HeaderSet(http.Header{
				"Set-Cookie": []string{"csrf_token=123456"},
			}),
		)

		httpmock.RegisterResponder(
			"POST",
			"http://mock.kratos/self-service/login",
			testutil.WithRequest(func(req *http.Request) (*http.Response, error) {
				req.ParseForm()
				// Return 400 error - not a redirect
				return httpmock.NewStringResponse(400, "Bad credentials"), nil
			}),
		)

		err := KratosLogin(config, mockClient)
		assert.ErrorContains(t, err, "failed to submit login")
		err = KratosLogin(config, mockClient) // Second call should also fail gracefully
		assert.ErrorContains(t, err, "failed to submit login")
	})

	t.Run("fails when totp form returns 400", func(t *testing.T) {
		mockClient := setupMockClient(t)

		// Use loginFlowResponder to simulate successful password redirect but failing 2FA
		httpmock.RegisterResponder(
			"GET",
			"http://mock.kratos/self-service/login/browser",
			testutil.WithRequest(httpmock.NewStringResponder(302, "").HeaderSet(http.Header{
				"Location": []string{"http://mock.kratos/example-login-page?flow=abc123-def456"},
			})),
		)

		httpmock.RegisterResponder(
			"GET",
			"http://mock.kratos/example-login-page",
			testutil.WithRequest(httpmock.NewStringResponder(200, "").HeaderSet(http.Header{
				"Set-Cookie": []string{"csrf_token=123456"},
			})),
		)

		// First POST: password succeeds and redirects to 2FA
		// Second POST: totp fails with 400
		httpmock.RegisterResponder(
			"POST",
			"http://mock.kratos/self-service/login",
			testutil.WithRequest(loginFlowResponder("http://mock.kratos/example-login-page?flow=abc123-def456", "")),
		)

		// GET handler for second flow page
		httpmock.RegisterResponder(
			"GET",
			"http://mock.kratos/example-login-page",
			testutil.WithRequest(httpmock.NewStringResponder(200, "").HeaderSet(http.Header{
				"Set-Cookie": []string{"csrf_token=987654"},
			})),
		)

		err := KratosLogin(config, mockClient)
		assert.ErrorContains(t, err, "failed to submit 2FA")
	})

	t.Run("fails when login does not redirect to /settings page", func(t *testing.T) {
		mockClient := setupMockClient(t)

		httpmock.RegisterResponder(
			"GET",
			"http://mock.kratos/self-service/login/browser",
			testutil.WithRequest(httpmock.NewStringResponder(302, "").HeaderSet(http.Header{
				"Location": []string{"http://mock.kratos/example-login-page?flow=abc123-def456"},
			})),
		)

		httpmock.RegisterResponder(
			"GET",
			"http://mock.kratos/example-login-page",
			testutil.WithRequest(httpmock.NewStringResponder(200, "").HeaderSet(http.Header{
				"Set-Cookie": []string{"csrf_token=123456"},
			})),
		)

		httpmock.RegisterResponder(
			"GET",
			"http://mock.kratos/logout",
			testutil.WithRequest(httpmock.NewStringResponder(200, "")),
		)

		// Password succeeds but redirects to unexpected page (not /settings)
		httpmock.RegisterResponder(
			"POST",
			"http://mock.kratos/self-service/login",
			testutil.WithRequest(loginFlowResponder("http://mock.kratos/example-login-page?flow=2fapage", "http://mock.kratos/logout")),
		)

		err := KratosLogin(config, mockClient)
		assert.ErrorContains(t, err, "Bad login")
	})

	t.Run("fails when login does not set session cookie", func(t *testing.T) {
		mockClient := setupMockClient(t)

		// Step 1a: Redirect to login page
		httpmock.RegisterResponder(
			"GET",
			"http://mock.kratos/self-service/login/browser",
			testutil.WithRequest(httpmock.NewStringResponder(302, "").HeaderSet(http.Header{
				"Location": []string{"http://mock.kratos/example-login-page?flow=abc123-def456"},
			})),
		)

		// Step 1b: Return login page with CSRF token
		httpmock.RegisterResponder(
			"GET",
			"http://mock.kratos/example-login-page",
			testutil.WithRequest(httpmock.NewStringResponder(200, "").HeaderSet(http.Header{
				"Set-Cookie": []string{"csrf_token=123456"},
			})),
		)

		// Step 2-3: Form submission (password and 2FA)
		httpmock.RegisterResponder(
			"POST",
			"http://mock.kratos/self-service/login",
			testutil.WithRequest(loginFlowResponder("http://mock.kratos/example-login-page?flow=2fapage", "http://mock.kratos/settings/public/uid/testuser")),
		)

		// Finish: Return settings page WITHOUT login cookie
		httpmock.RegisterResponder(
			"GET",
			"http://mock.kratos/settings/public/uid/testuser",
			testutil.WithRequest(httpmock.NewStringResponder(200, "<html><body>you did it</body></html>")),
		)

		err := KratosLogin(config, mockClient)
		assert.ErrorContains(t, err, "Missing session cookie")
	})
}
