package hytale

import (
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
		httpmock.RegisterResponder("GET", config.Feeds.Patchlines, httpmock.NewStringResponder(200, testutil.SamplePatchlinesResponse))
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
		httpmock.RegisterResponder("GET", config.Auth.LauncherData, httpmock.NewStringResponder(200, testutil.SampleLauncherData))
		authStore := atestutil.NewTestAuthStore(auth.Launcher)

		profile, err := GetPatchlines(config, http, authStore)
		assert.NoError(t, err)
		td.Cmp(t, profile, expectedPatchlines)
	})

}
