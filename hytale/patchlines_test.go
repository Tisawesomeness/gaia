package hytale

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/Tisawesomeness/gaia/auth"
	"github.com/Tisawesomeness/gaia/config"
	"github.com/Tisawesomeness/gaia/testutil/atestutil"
	"github.com/Tisawesomeness/gaia/testutil/testutil"
	"github.com/jarcoal/httpmock"
	"github.com/maxatome/go-testdeep/td"
	"github.com/stretchr/testify/assert"
)

var (
	expectedPatchlines = map[string]*time.Time{
		"release":     nil,
		"pre-release": nil,
		"v0.4":        nil,
	}
	expiryTime                   = time.Unix(1782691481, 0)
	expectedPatchlinesWithExpiry = map[string]*time.Time{
		"release": &expiryTime,
	}
)

func TestClosestPatchline(t *testing.T) {
	samplePatchlines := map[string]*time.Time{
		"release":     nil,
		"pre-release": nil,
		"v0.4":        nil,
		"v0.4.1":      nil,
	}

	testCases := []struct {
		input    string
		expected string
	}{
		{"Release", "release"},
		{"release", "release"},
		{"Pre-Release", "pre-release"},
		{"Pre release", "pre-release"},
		{"pre_release", "pre-release"},
		{"pre-release", "pre-release"},
		{"v0.4", "v0.4"},
		{"V0.4", "v0.4"},
		{"0.4", "v0.4"},
		{"0.40", "v0.4"},
		{"v0.4.1", "v0.4.1"},
		{"v0.40.10", "v0.4.1"},
		{"rellease", ""},
		{"v0.41", ""},
		{"v0.4+abc", ""},
		{"xyz", ""},
		{"", ""},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("ClosestPatchline(%q) = %q", tc.input, tc.expected), func(t *testing.T) {
			result := ClosestPatchline(tc.input, samplePatchlines)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestPatchlines(t *testing.T) {
	http := &http.Client{
		Timeout: time.Duration(10) * time.Second,
	}
	httpmock.ActivateNonDefault(http)

	config := &config.Config{
		Auth: config.AuthConfig{
			LauncherData: "https://account-data.example.com/my-account/get-launcher-data?arch=amd64&os=windows",
		},
		Feeds: config.FeedsConfig{
			Patchlines: "https://account-data.example.com/my-account/get-patchlines",
		},
	}

	t.Run("fetch patchlines", func(t *testing.T) {
		httpmock.RegisterResponder("GET", config.Feeds.Patchlines, httpmock.NewStringResponder(200, testutil.SamplePatchlinesResponseWithV0_4))
		authStore := atestutil.NewTestAuthStore(auth.Server)

		profile, err := GetPatchlines(config, http, authStore)
		assert.NoError(t, err)
		td.Cmp(t, profile, expectedPatchlines)
	})

	t.Run("fetch patchlines with expiry", func(t *testing.T) {
		httpmock.RegisterResponder("GET", config.Feeds.Patchlines, httpmock.NewStringResponder(200, testutil.SamplePatchlinesResponseExpiry))
		authStore := atestutil.NewTestAuthStore(auth.Server)

		profile, err := GetPatchlines(config, http, authStore)
		assert.NoError(t, err)
		td.Cmp(t, profile, expectedPatchlinesWithExpiry)
	})

	t.Run("fetch patchlines 500", func(t *testing.T) {
		httpmock.RegisterResponder("GET", config.Feeds.Patchlines, httpmock.NewStringResponder(500, ""))
		authStore := atestutil.NewTestAuthStore(auth.Server)

		_, err := GetPatchlines(config, http, authStore)
		assert.Error(t, err)
	})

	t.Run("fetch patchlines, launcher auth redirects to launcher data", func(t *testing.T) {
		httpmock.RegisterResponder("GET", config.Auth.LauncherData, httpmock.NewStringResponder(200, testutil.SampleLauncherDataWithV0_4))
		authStore := atestutil.NewTestAuthStore(auth.Launcher)

		profile, err := GetPatchlines(config, http, authStore)
		assert.NoError(t, err)
		td.Cmp(t, profile, expectedPatchlines)
	})

}
