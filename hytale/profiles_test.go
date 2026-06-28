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
	sampleUUID     = "d798091b-f494-4208-a1ba-e24da5880786"
	sampleUsername = "tis"
	expected       = &PublicGameProfile{
		UUID:     sampleUUID,
		Username: sampleUsername,
		Skin: toStringRefMap(map[string]string{
			"bodyCharacteristic": "Muscular.10",
			"underwear":          "Suit.Blue",
			"face":               "Face_Almond_Eyes",
			"ears":               "Elf_Ears",
			"mouth":              "Mouth_Default",
			"haircut":            "SuperSlickback.BlondCaramel",
			"facialHair":         "",
			"eyebrows":           "",
			"eyes":               "Almond_Eyes.Grey",
			"pants":              "CostumePants.Black",
			"overpants":          "LongSocks_Bow.Charcoal",
			"undertop":           "LongSleeveShirt_ButtonUp.Lime",
			"overtop":            "JacketLong.Red",
			"shoes":              "",
			"headAccessory":      "",
			"faceAccessory":      "",
			"earAccessory":       "",
			"skinFeature":        "",
			"gloves":             "",
			"cape":               "",
		}),
	}
)

func toStringRefMap(m map[string]string) map[string]*string {
	mr := make(map[string]*string)
	for k, v := range m {
		if v == "" {
			mr[k] = nil
		} else {
			mr[k] = &v
		}
	}
	return mr
}

func TestProfiles(t *testing.T) {
	http := &http.Client{
		Timeout: time.Duration(10) * time.Second,
	}
	httpmock.ActivateNonDefault(http)

	config := &config.Config{
		Profile: config.ProfileConfig{
			ByUUID:     "https://account-data.example.com/profile/uuid/",
			ByUsername: "https://account-data.example.com/profile/username/",
		},
	}

	authStore := atestutil.NewTestAuthStore(auth.Server)

	t.Run("fetch profile by username", func(t *testing.T) {
		httpmock.RegisterResponder("GET", config.Profile.ByUsername+sampleUsername, httpmock.NewStringResponder(200, testutil.SampleProfileResponse))
		profile, err := FetchProfileFromUsername(config, http, authStore, sampleUsername)
		assert.NoError(t, err)
		td.Cmp(t, profile, expected)
	})

	t.Run("fetch profile by username 404", func(t *testing.T) {
		httpmock.RegisterResponder("GET", config.Profile.ByUsername+sampleUsername, httpmock.NewStringResponder(404, ""))
		profile, err := FetchProfileFromUsername(config, http, authStore, sampleUsername)
		assert.NoError(t, err)
		td.Cmp(t, profile, td.Nil())
	})

	t.Run("fetch profile by uuid", func(t *testing.T) {
		httpmock.RegisterResponder("GET", config.Profile.ByUUID+sampleUUID, httpmock.NewStringResponder(200, testutil.SampleProfileResponse))
		profile, err := FetchProfileFromUUID(config, http, authStore, sampleUUID)
		assert.NoError(t, err)
		td.Cmp(t, profile, expected)
	})

	t.Run("fetch profile by uuid 404", func(t *testing.T) {
		httpmock.RegisterResponder("GET", config.Profile.ByUUID+sampleUUID, httpmock.NewStringResponder(404, ""))
		profile, err := FetchProfileFromUUID(config, http, authStore, sampleUUID)
		assert.NoError(t, err)
		td.Cmp(t, profile, td.Nil())
	})
}
