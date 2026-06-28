package db

import (
	"context"
	"testing"
	"time"

	"github.com/Tisawesomeness/gaia/config"
	"github.com/Tisawesomeness/gaia/testutil/testutil"
	"github.com/stretchr/testify/assert"
)

var (
	testDB   *DB
	testCase = testutil.MakeTestCase(beforeEach, nil)
)

func init() {
	db, err := NewDB(config.ValkeyConfig{
		Address:       "127.0.0.1",
		Port:          9999,
		DatabaseIndex: 1,
	})
	if err != nil {
		panic(err)
	}
	testDB = db
}

func teardown() {
	testDB.Close()
}

func beforeEach() {
	testDB.Clear()
}

func removeAuthType(key string) error {
	command := testDB.v.B().Hdel().Key(key).Field("auth_type").Build()
	return testDB.v.Do(context.Background(), command).Error()
}

func TestDB(t *testing.T) {
	t.Cleanup(teardown)

	t.Run("add user subscription", testCase(func(t *testing.T) {
		testDB.AddOrUpdateSubscription("user_feed", "user_target", UserSubscription{Version: "v1"})
		_, err := testDB.GetSubscription("user_feed", "user_target")
		assert.NoError(t, err)
	}))

	t.Run("get user subscription", testCase(func(t *testing.T) {
		testDB.AddOrUpdateSubscription("user_feed_2", "user_target_2", UserSubscription{Version: "v1"})
		result, err := testDB.GetSubscription("user_feed_2", "user_target_2")
		assert.NoError(t, err)
		assert.IsType(t, UserSubscription{}, result)
	}))

	t.Run("remove user subscription", testCase(func(t *testing.T) {
		testDB.AddOrUpdateSubscription("user_feed_3", "user_target_3", UserSubscription{Version: "v1"})
		testDB.RemoveSubscription("user_feed_3", "user_target_3")
		_, err := testDB.GetSubscription("user_feed_3", "user_target_3")
		assert.Error(t, err)
	}))

	t.Run("edge case: get non-existent", testCase(func(t *testing.T) {
		_, err := testDB.GetSubscription("x", "y")
		assert.Error(t, err)
	}))

	t.Run("edge case: remove non-existent", testCase(func(t *testing.T) {
		err := testDB.RemoveSubscription("x", "y")
		assert.NoError(t, err)
	}))

	t.Run("add guild subscription", testCase(func(t *testing.T) {
		testDB.AddOrUpdateSubscription("guild_feed", "guild_target", GuildSubscription{Version: "latest", Roles: []string{"r1", "r2"}})
		result, err := testDB.GetSubscription("guild_feed", "guild_target")
		assert.NoError(t, err)
		assert.IsType(t, GuildSubscription{}, result)
		gs := result.(GuildSubscription)
		assert.Len(t, gs.Roles, 2)
	}))

	t.Run("get guild subscriptions", testCase(func(t *testing.T) {
		for _, g := range []string{"a", "b", "c"} {
			testDB.AddOrUpdateSubscription("guild_feed_2", g, GuildSubscription{Version: "l"})
		}
		subs, _ := testDB.GetSubscriptions("guild_feed_2")
		assert.Len(t, subs, 3)
	}))

	t.Run("remove guild subscription", testCase(func(t *testing.T) {
		testDB.AddOrUpdateSubscription("guild_feed_3", "guild_x", GuildSubscription{Version: "l"})
		testDB.RemoveSubscription("guild_feed_3", "guild_x")
		_, err := testDB.GetSubscription("guild_feed_3", "guild_x")
		assert.Error(t, err)
	}))

	t.Run("get non-existent guild subscription", testCase(func(t *testing.T) {
		_, err := testDB.GetSubscription("nonexistent", "nonexistent")
		assert.Error(t, err)
	}))

	t.Run("mixed user and guild subscriptions", testCase(func(t *testing.T) {
		testDB.AddOrUpdateSubscription("mixed_feed", "user1", UserSubscription{Version: "l"})
		testDB.AddOrUpdateSubscription("mixed_feed", "guild1", GuildSubscription{Version: "l", Roles: []string{"r"}})
		result, err := testDB.GetSubscription("mixed_feed", "user1")
		assert.NoError(t, err)
		assert.IsType(t, UserSubscription{}, result)
		result, err = testDB.GetSubscription("mixed_feed", "guild1")
		assert.NoError(t, err)
		assert.IsType(t, GuildSubscription{}, result)
	}))

	t.Run("add same target twice", testCase(func(t *testing.T) {
		testDB.AddOrUpdateSubscription("dup_feed", "t", UserSubscription{})
		testDB.AddOrUpdateSubscription("dup_feed", "t", UserSubscription{})
		subs, _ := testDB.GetSubscriptions("dup_feed")
		assert.Len(t, subs, 1)
	}))

	t.Run("remove last subscription clears feed", testCase(func(t *testing.T) {
		testDB.AddOrUpdateSubscription("clear_feed", "t", UserSubscription{Version: "v"})
		testDB.RemoveSubscription("clear_feed", "t")
		subs, _ := testDB.GetSubscriptions("clear_feed")
		assert.Len(t, subs, 0)
	}))

	t.Run("guild subscription type", testCase(func(t *testing.T) {
		testDB.AddOrUpdateSubscription("type_guild", "target", GuildSubscription{Version: "l", Roles: []string{"r1", "r2", "r3"}})
		result, err := testDB.GetSubscription("type_guild", "target")
		assert.NoError(t, err)
		assert.IsType(t, GuildSubscription{}, result)
		guildSub := result.(GuildSubscription)
		assert.Equal(t, "guild", guildSub.Type())
		assert.Equal(t, "l", guildSub.CurrentVersion())
		assert.Len(t, guildSub.Roles, 3)
	}))

	t.Run("user subscription type", testCase(func(t *testing.T) {
		testDB.AddOrUpdateSubscription("type_user", "target", UserSubscription{Version: "l"})
		result, err := testDB.GetSubscription("type_user", "target")
		assert.NoError(t, err)
		assert.IsType(t, UserSubscription{}, result)
		userSub := result.(UserSubscription)
		assert.Equal(t, "user", userSub.Type())
		assert.Equal(t, "l", userSub.CurrentVersion())
	}))

	t.Run("subscribe to feed", testCase(func(t *testing.T) {
		testDB.AddOrUpdateSubscription("sub_feed", "sub_target", UserSubscription{Version: "v1"})
		result, err := testDB.GetSubscription("sub_feed", "sub_target")
		assert.NoError(t, err)
		assert.IsType(t, UserSubscription{}, result)
		userSub := result.(UserSubscription)
		assert.Equal(t, "v1", userSub.Version)
	}))

	t.Run("unsubscribe from feed", testCase(func(t *testing.T) {
		testDB.AddOrUpdateSubscription("sub_feed", "sub_target", UserSubscription{Version: "v"})
		testDB.RemoveSubscription("sub_feed", "sub_target")
		subs, _ := testDB.GetSubscriptions("sub_feed")
		assert.Len(t, subs, 0)
	}))

	t.Run("remove non-existent subscription", testCase(func(t *testing.T) {
		err := testDB.RemoveSubscription("nonexistent", "nonexistent")
		assert.NoError(t, err)
	}))

	t.Run("get latest post", testCase(func(t *testing.T) {
		testDB.SetLatestPost("latest_feed", "post content")
		result, _ := testDB.GetLatestPost("latest_feed")
		assert.Equal(t, "post content", string(result))
	}))

	t.Run("get latest post for non-existent", testCase(func(t *testing.T) {
		result, _ := testDB.GetLatestPost("nonexistent_post")
		assert.Empty(t, string(result))
	}))

	t.Run("get oauth token when set", testCase(func(t *testing.T) {
		token := OAuthToken{
			AccessToken:  "access123",
			RefreshToken: "refresh456",
			ExpiresAt:    time.Now().Add(time.Hour),
			AuthType:     "launcher",
		}
		err := testDB.SetOAuthToken(token)
		assert.NoError(t, err)
		result, err := testDB.GetOAuthToken()
		assert.NoError(t, err)
		assert.Equal(t, "access123", result.AccessToken)
		assert.Equal(t, "refresh456", result.RefreshToken)
		assert.Equal(t, "launcher", result.AuthType)
	}))

	t.Run("get oauth token returns nil when not set", testCase(func(t *testing.T) {
		result, err := testDB.GetOAuthToken()
		assert.NoError(t, err)
		assert.Nil(t, result)
	}))

	t.Run("set oauth token with empty values", testCase(func(t *testing.T) {
		token := OAuthToken{
			AccessToken:  "",
			RefreshToken: "",
			ExpiresAt:    time.Now().Add(time.Hour),
			AuthType:     "launcher",
		}
		err := testDB.SetOAuthToken(token)
		assert.NoError(t, err)
		result, err := testDB.GetOAuthToken()
		assert.NoError(t, err)
		assert.Empty(t, result.AccessToken)
		assert.Empty(t, result.RefreshToken)
	}))

	t.Run("migrate oauth without auth type", testCase(func(t *testing.T) {
		token := OAuthToken{
			AccessToken:  "access123",
			RefreshToken: "refresh456",
			ExpiresAt:    time.Now().Add(time.Hour),
		}
		err := testDB.SetOAuthToken(token)
		assert.NoError(t, err)
		err = removeAuthType("oauth_token")
		assert.NoError(t, err)
		result, err := testDB.GetOAuthToken()
		assert.NoError(t, err)
		assert.Equal(t, "server", result.AuthType)
	}))

	t.Run("get profile uuid when set", testCase(func(t *testing.T) {
		uuid := "abc123-def456"
		err := testDB.SetProfileUUID(uuid)
		assert.NoError(t, err)
		result, err := testDB.GetProfileUUID()
		assert.NoError(t, err)
		assert.Equal(t, uuid, result)
	}))

	t.Run("get profile uuid returns empty when not set", testCase(func(t *testing.T) {
		result, err := testDB.GetProfileUUID()
		assert.NoError(t, err)
		assert.Empty(t, result)
	}))

	t.Run("set get game session", testCase(func(t *testing.T) {
		session := GameSessionToken{
			SessionToken: "session789",
			ExpiresAt:    time.Now().Add(2 * time.Hour),
			AuthType:     "server",
		}
		err := testDB.SetGameSession(session)
		assert.NoError(t, err)
		result, err := testDB.GetGameSession()
		assert.NoError(t, err)
		assert.Equal(t, "session789", result.SessionToken)
		assert.Equal(t, "server", result.AuthType)
	}))

	t.Run("get game session returns nil when not set", testCase(func(t *testing.T) {
		result, err := testDB.GetGameSession()
		assert.NoError(t, err)
		assert.Nil(t, result)
	}))

	t.Run("migrate game session without auth type", testCase(func(t *testing.T) {
		session := GameSessionToken{
			SessionToken: "session789",
			ExpiresAt:    time.Now().Add(2 * time.Hour),
		}
		err := testDB.SetGameSession(session)
		assert.NoError(t, err)
		err = removeAuthType("game_session")
		assert.NoError(t, err)
		result, err := testDB.GetGameSession()
		assert.NoError(t, err)
		assert.Equal(t, "server", result.AuthType)
	}))

	t.Run("overwrite oauth token replaces all fields", testCase(func(t *testing.T) {
		oldToken := OAuthToken{
			AccessToken:  "old_access",
			RefreshToken: "old_refresh",
			ExpiresAt:    time.Now().Add(time.Hour),
			AuthType:     "launcher",
		}
		testDB.SetOAuthToken(oldToken)

		newToken := OAuthToken{
			AccessToken: "new_access",
			ExpiresAt:   time.Now().Add(2 * time.Hour),
			AuthType:    "server",
		}
		testDB.SetOAuthToken(newToken)

		result, _ := testDB.GetOAuthToken()
		assert.Equal(t, "new_access", result.AccessToken)
		assert.Equal(t, "", result.RefreshToken)
		assert.Equal(t, newToken.ExpiresAt.Unix(), result.ExpiresAt.Unix())
		assert.Equal(t, "server", result.AuthType)
	}))
}
