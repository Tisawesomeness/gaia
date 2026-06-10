package hytale

import (
	"net/http"
	"testing"
	"time"

	"github.com/Tisawesomeness/gaia/config"
	"github.com/Tisawesomeness/gaia/testutil"
	"github.com/jarcoal/httpmock"
	"github.com/maxatome/go-testdeep/td"
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
			ByUUID:       "https://account-data.example.com/profile/uuid/",
			ByUsername:   "https://account-data.example.com/profile/username/",
			Availability: "https://accounts.example.com/api/account/username-reservations/availability",
		},
	}

	authStore := testutil.NewTestAuthStore(http)
	kratosClient, ok := authStore.GetKratosClient()
	if !ok {
		t.Fatalf("Could not get kratos client")
	}

	t.Run("fetch profile by username", func(t *testing.T) {
		httpmock.RegisterResponder("GET", config.Profile.ByUsername+sampleUsername, httpmock.NewStringResponder(200, testutil.SampleProfileResponse))
		profile, err := FetchProfileFromUsername(sampleUsername, config, http, authStore)
		if err != nil {
			t.Fatalf("%v", err)
		}
		td.Cmp(t, profile, expected)
	})

	t.Run("fetch profile by username 404", func(t *testing.T) {
		httpmock.RegisterResponder("GET", config.Profile.ByUsername+sampleUsername, httpmock.NewStringResponder(404, ""))
		profile, err := FetchProfileFromUsername(sampleUsername, config, http, authStore)
		if err != nil {
			t.Fatalf("%v", err)
		}
		td.Cmp(t, profile, td.Nil())
	})

	t.Run("fetch profile by uuid", func(t *testing.T) {
		httpmock.RegisterResponder("GET", config.Profile.ByUUID+sampleUUID, httpmock.NewStringResponder(200, testutil.SampleProfileResponse))
		profile, err := FetchProfileFromUUID(sampleUUID, config, http, authStore)
		if err != nil {
			t.Fatalf("%v", err)
		}
		td.Cmp(t, profile, expected)
	})

	t.Run("fetch profile by uuid 404", func(t *testing.T) {
		httpmock.RegisterResponder("GET", config.Profile.ByUUID+sampleUUID, httpmock.NewStringResponder(404, ""))
		profile, err := FetchProfileFromUUID(sampleUUID, config, http, authStore)
		if err != nil {
			t.Fatalf("%v", err)
		}
		td.Cmp(t, profile, td.Nil())
	})

	t.Run("username availability", func(t *testing.T) {
		httpmock.RegisterResponder("GET", config.Profile.Availability, httpmock.NewStringResponder(200, ""))
		availability, err := CheckAvailability(sampleUsername, config, kratosClient)
		if err != nil {
			t.Fatalf("%v", err)
		}
		td.Cmp(t, availability, Available)
	})

	t.Run("username availability reserved", func(t *testing.T) {
		httpmock.RegisterResponder("GET", config.Profile.Availability, httpmock.NewStringResponder(200, "reserved by the Hytale Team"))
		availability, err := CheckAvailability(sampleUsername, config, kratosClient)
		if err != nil {
			t.Fatalf("%v", err)
		}
		td.Cmp(t, availability, HytaleReserved)
	})

	t.Run("username availability 400", func(t *testing.T) {
		httpmock.RegisterResponder("GET", config.Profile.Availability, httpmock.NewStringResponder(400, "Username is already reserved"))
		availability, err := CheckAvailability(sampleUsername, config, kratosClient)
		if err != nil {
			t.Fatalf("%v", err)
		}
		td.Cmp(t, availability, Reserved)
	})
}
