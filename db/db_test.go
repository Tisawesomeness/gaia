package db

import (
	"testing"
	"time"

	"github.com/Tisawesomeness/gaia/config"
	"github.com/Tisawesomeness/gaia/testutil"
)

var (
	testDB   *DB
	testCase = testutil.MakeTestCase(beforeEach, nil)
)

func init() {
	db, err := NewDB(config.ValkeyConfig{
		Address: "127.0.0.1",
		Port:    9999,
	})
	if err != nil {
		panic(err)
	}
	testDB = db
	testDB.ClearAll()
}

func teardown() {
	testDB.Close()
}

func beforeEach() {
	testDB.ClearAll()
}

func TestDB(t *testing.T) {
	t.Cleanup(teardown)

	t.Run("add user subscription", testCase(func(t *testing.T) {
		testDB.AddOrUpdateSubscription("user_feed", "user_target", UserSubscription{Version: "v1"})
		_, err := testDB.GetSubscription("user_feed", "user_target")
		if err != nil {
			t.Fatal(err)
		}
	}))

	t.Run("get user subscription", testCase(func(t *testing.T) {
		testDB.AddOrUpdateSubscription("user_feed_2", "user_target_2", UserSubscription{Version: "v1"})
		result, err := testDB.GetSubscription("user_feed_2", "user_target_2")
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := result.(UserSubscription); !ok {
			t.Fatal("expected UserSubscription")
		}
	}))

	t.Run("remove user subscription", testCase(func(t *testing.T) {
		testDB.AddOrUpdateSubscription("user_feed_3", "user_target_3", UserSubscription{Version: "v1"})
		testDB.RemoveSubscription("user_feed_3", "user_target_3")
		_, err := testDB.GetSubscription("user_feed_3", "user_target_3")
		if err == nil {
			t.Error("expected error after remove")
		}
	}))

	t.Run("edge case: get non-existent", testCase(func(t *testing.T) {
		_, err := testDB.GetSubscription("x", "y")
		if err == nil {
			t.Error("expected error for non-existent subscription")
		}
	}))

	t.Run("edge case: remove non-existent", testCase(func(t *testing.T) {
		err := testDB.RemoveSubscription("x", "y")
		if err != nil {
			t.Errorf("expected no error: %v", err)
		}
	}))

	t.Run("add guild subscription", testCase(func(t *testing.T) {
		testDB.AddOrUpdateSubscription("guild_feed", "guild_target", GuildSubscription{Version: "latest", Roles: []string{"r1", "r2"}})
		result, err := testDB.GetSubscription("guild_feed", "guild_target")
		if err != nil {
			t.Fatal(err)
		}
		if guildSub, ok := result.(GuildSubscription); ok {
			if len(guildSub.Roles) != 2 {
				t.Errorf("Roles length = %d, want 2", len(guildSub.Roles))
			}
		}
	}))

	t.Run("get guild subscriptions", testCase(func(t *testing.T) {
		for _, g := range []string{"a", "b", "c"} {
			testDB.AddOrUpdateSubscription("guild_feed_2", g, GuildSubscription{Version: "l"})
		}
		subs, _ := testDB.GetSubscriptions("guild_feed_2")
		if len(subs) != 3 {
			t.Errorf("Got %d subs, want 3", len(subs))
		}
	}))

	t.Run("remove guild subscription", testCase(func(t *testing.T) {
		testDB.AddOrUpdateSubscription("guild_feed_3", "guild_x", GuildSubscription{Version: "l"})
		testDB.RemoveSubscription("guild_feed_3", "guild_x")
		_, err := testDB.GetSubscription("guild_feed_3", "guild_x")
		if err == nil {
			t.Error("expected error after remove")
		}
	}))

	t.Run("get non-existent guild subscription", testCase(func(t *testing.T) {
		_, err := testDB.GetSubscription("nonexistent", "nonexistent")
		if err == nil {
			t.Error("expected error for non-existent guild")
		}
	}))

	t.Run("mixed user and guild subscriptions", testCase(func(t *testing.T) {
		testDB.AddOrUpdateSubscription("mixed_feed", "user1", UserSubscription{Version: "l"})
		testDB.AddOrUpdateSubscription("mixed_feed", "guild1", GuildSubscription{Version: "l", Roles: []string{"r"}})
		result, err := testDB.GetSubscription("mixed_feed", "user1")
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := result.(UserSubscription); !ok {
			t.Error("expected UserSubscription")
		}
		result, err = testDB.GetSubscription("mixed_feed", "guild1")
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := result.(GuildSubscription); !ok {
			t.Error("expected GuildSubscription")
		}
	}))

	t.Run("add same target twice", testCase(func(t *testing.T) {
		testDB.AddOrUpdateSubscription("dup_feed", "t", UserSubscription{})
		testDB.AddOrUpdateSubscription("dup_feed", "t", UserSubscription{})
		subs, _ := testDB.GetSubscriptions("dup_feed")
		if len(subs) != 1 {
			t.Errorf("Got %d subs, want 1", len(subs))
		}
	}))

	t.Run("remove last subscription clears feed", testCase(func(t *testing.T) {
		testDB.AddOrUpdateSubscription("clear_feed", "t", UserSubscription{Version: "v"})
		testDB.RemoveSubscription("clear_feed", "t")
		subs, _ := testDB.GetSubscriptions("clear_feed")
		if len(subs) != 0 {
			t.Errorf("Got %d subs, want 0", len(subs))
		}
	}))

	t.Run("guild subscription type", testCase(func(t *testing.T) {
		testDB.AddOrUpdateSubscription("type_guild", "target", GuildSubscription{Version: "l", Roles: []string{"r1", "r2", "r3"}})
		result, err := testDB.GetSubscription("type_guild", "target")
		if err != nil {
			t.Fatal(err)
		}
		if guildSub, ok := result.(GuildSubscription); ok {
			if guildSub.Type() != "guild" {
				t.Error("expected Type() == guild")
			}
			if guildSub.CurrentVersion() != "l" {
				t.Error("expected CurrentVersion() == l")
			}
			if len(guildSub.Roles) != 3 {
				t.Errorf("Got %d roles, want 3", len(guildSub.Roles))
			}
		}
	}))

	t.Run("user subscription type", testCase(func(t *testing.T) {
		testDB.AddOrUpdateSubscription("type_user", "target", UserSubscription{Version: "l"})
		result, err := testDB.GetSubscription("type_user", "target")
		if err != nil {
			t.Fatal(err)
		}
		if userSub, ok := result.(UserSubscription); ok {
			if userSub.Type() != "user" {
				t.Error("expected Type() == user")
			}
			if userSub.CurrentVersion() != "l" {
				t.Error("expected CurrentVersion() == l")
			}
		}
	}))

	t.Run("subscribe to feed", testCase(func(t *testing.T) {
		testDB.AddOrUpdateSubscription("sub_feed", "sub_target", UserSubscription{Version: "v1"})
		result, err := testDB.GetSubscription("sub_feed", "sub_target")
		if err != nil {
			t.Fatal(err)
		}
		if userSub, ok := result.(UserSubscription); ok {
			if userSub.Version != "v1" {
				t.Errorf("Got version %q, want v1", userSub.Version)
			}
		}
	}))

	t.Run("unsubscribe from feed", testCase(func(t *testing.T) {
		testDB.AddOrUpdateSubscription("sub_feed", "sub_target", UserSubscription{Version: "v"})
		testDB.RemoveSubscription("sub_feed", "sub_target")
		subs, _ := testDB.GetSubscriptions("sub_feed")
		if len(subs) != 0 {
			t.Errorf("Got %d subs, want 0", len(subs))
		}
	}))

	t.Run("remove non-existent subscription", testCase(func(t *testing.T) {
		err := testDB.RemoveSubscription("nonexistent", "nonexistent")
		if err != nil {
			t.Errorf("expected no error: %v", err)
		}
	}))

	t.Run("get latest post", testCase(func(t *testing.T) {
		testDB.SetLatestPost("latest_feed", "post content")
		result, _ := testDB.GetLatestPost("latest_feed")
		if string(result) != "post content" {
			t.Errorf("Got %q, want post content", string(result))
		}
	}))

	t.Run("get latest post for non-existent", testCase(func(t *testing.T) {
		result, _ := testDB.GetLatestPost("nonexistent_post")
		if string(result) != "" {
			t.Errorf("Got %q, want empty", string(result))
		}
	}))

	t.Run("get oauth token when set", testCase(func(t *testing.T) {
		token := OAuthToken{
			AccessToken:  "access123",
			RefreshToken: "refresh456",
			ExpiresAt:    time.Now().Add(time.Hour),
		}
		err := testDB.SetOAuthToken(token)
		if err != nil {
			t.Fatal(err)
		}
		result, err := testDB.GetOAuthToken()
		if err != nil {
			t.Fatal(err)
		}
		if result.AccessToken != "access123" {
			t.Errorf("Got %q, want access123", result.AccessToken)
		}
		if result.RefreshToken != "refresh456" {
			t.Errorf("Got %q, want refresh456", result.RefreshToken)
		}
	}))

	t.Run("get oauth token returns nil when not set", testCase(func(t *testing.T) {
		result, err := testDB.GetOAuthToken()
		if err != nil {
			t.Fatal(err)
		}
		if result != nil {
			t.Error("expected nil when not set")
		}
	}))

	t.Run("set oauth token with empty values", testCase(func(t *testing.T) {
		token := OAuthToken{
			AccessToken:  "",
			RefreshToken: "",
			ExpiresAt:    time.Now().Add(time.Hour),
		}
		err := testDB.SetOAuthToken(token)
		if err != nil {
			t.Fatal(err)
		}
		result, err := testDB.GetOAuthToken()
		if err != nil {
			t.Fatal(err)
		}
		if result.AccessToken != "" {
			t.Errorf("Got %q, want empty", result.AccessToken)
		}
		if result.RefreshToken != "" {
			t.Errorf("Got %q, want empty", result.RefreshToken)
		}
	}))

	t.Run("get profile uuid when set", testCase(func(t *testing.T) {
		uuid := "abc123-def456"
		err := testDB.SetProfileUUID(uuid)
		if err != nil {
			t.Fatal(err)
		}
		result, err := testDB.GetProfileUUID()
		if err != nil {
			t.Fatal(err)
		}
		if result != uuid {
			t.Errorf("Got %q, want %s", result, uuid)
		}
	}))

	t.Run("get profile uuid returns empty when not set", testCase(func(t *testing.T) {
		result, err := testDB.GetProfileUUID()
		if err != nil {
			t.Fatal(err)
		}
		if result != "" {
			t.Errorf("Got %q, want empty", result)
		}
	}))

	t.Run("set get game session", testCase(func(t *testing.T) {
		session := GameSessionToken{
			SessionToken: "session789",
			ExpiresAt:    time.Now().Add(2 * time.Hour),
		}
		err := testDB.SetGameSession(session)
		if err != nil {
			t.Fatal(err)
		}
		result, err := testDB.GetGameSession()
		if err != nil {
			t.Fatal(err)
		}
		if result.SessionToken != "session789" {
			t.Errorf("Got %q, want session789", result.SessionToken)
		}
	}))

	t.Run("get game session returns nil when not set", testCase(func(t *testing.T) {
		result, err := testDB.GetGameSession()
		if err != nil {
			t.Fatal(err)
		}
		if result != nil {
			t.Error("expected nil when not set")
		}
	}))

	t.Run("set get kratos refresh", testCase(func(t *testing.T) {
		refreshTime := time.Now().Add(time.Minute)
		err := testDB.SetKratosRefresh(refreshTime)
		if err != nil {
			t.Fatal(err)
		}
		result, err := testDB.GetKratosRefresh()
		if err != nil {
			t.Fatal(err)
		}
		expectedUnix := refreshTime.Unix()
		actualUnix := result.Unix()
		if expectedUnix != actualUnix {
			t.Errorf("Got unix %d, want %d", actualUnix, expectedUnix)
		}
	}))

	t.Run("get kratos refresh returns nil when not set", testCase(func(t *testing.T) {
		result, err := testDB.GetKratosRefresh()
		if err != nil {
			t.Fatal(err)
		}
		if result != nil {
			t.Error("expected nil when not set")
		}
	}))

	t.Run("overwrite oauth token replaces all fields", testCase(func(t *testing.T) {
		oldToken := OAuthToken{
			AccessToken:  "old_access",
			RefreshToken: "old_refresh",
			ExpiresAt:    time.Now().Add(time.Hour),
		}
		testDB.SetOAuthToken(oldToken)

		newToken := OAuthToken{
			AccessToken: "new_access",
			ExpiresAt:   time.Now().Add(2 * time.Hour),
		}
		testDB.SetOAuthToken(newToken)

		result, _ := testDB.GetOAuthToken()
		if result.AccessToken != "new_access" {
			t.Errorf("Got %q, want new_access", result.AccessToken)
		}
		if result.RefreshToken != "" {
			t.Errorf("Got %q, want empty", result.RefreshToken)
		}
		if result.ExpiresAt.Unix() != newToken.ExpiresAt.Unix() {
			t.Errorf("Got unix %d, want %d", result.ExpiresAt.Unix(), newToken.ExpiresAt.Unix())
		}
	}))
}
