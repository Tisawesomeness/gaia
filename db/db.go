package db

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Tisawesomeness/gaia/config"
	"github.com/valkey-io/valkey-go"
)

type DB struct {
	v valkey.Client
}

func NewDB(config config.ValkeyConfig) (*DB, error) {
	options := valkey.ClientOption{
		InitAddress: []string{config.Address + ":" + strconv.Itoa(config.Port)},
		SelectDB:    config.DatabaseIndex,
	}
	if config.Password != "" {
		options.Password = config.Password
	}
	client, err := valkey.NewClient(options)
	if err != nil {
		return nil, err
	}
	return &DB{client}, nil
}

func (db DB) Close() {
	db.v.Close()
}

// Prevents keys with ":" from being confused as sub-keys
// More of a sanity check than a security measure
func sanitize(keyPart string) string {
	return strings.ReplaceAll(keyPart, ":", ".")
}

// A subscription that can either be a UserSubscription or GuildSubscription
type Subscription interface {
	Type() string
	CurrentVersion() string
}

type GuildSubscription struct {
	Version string
	Roles   []string
}

func (GuildSubscription) Type() string {
	return "guild"
}
func (gs GuildSubscription) CurrentVersion() string {
	return gs.Version
}

type UserSubscription struct {
	Version string
}

func (UserSubscription) Type() string {
	return "user"
}
func (us UserSubscription) CurrentVersion() string {
	return us.Version
}

func (db DB) AddOrUpdateSubscription(feedID string, targetID string, subscription Subscription) error {
	key := sanitize(feedID) + ":subs:" + targetID

	// hset <feedID>:subs:<targetID> ...
	switch sub := subscription.(type) {
	case GuildSubscription:
		rolesStr := strings.Join(sub.Roles, ",")
		command := db.v.B().Hset().Key(key).FieldValue().
			FieldValue("type", sub.Type()).
			FieldValue("version", sub.CurrentVersion()).
			FieldValue("roles", rolesStr).
			Build()
		err := db.v.Do(context.Background(), command).Error()
		if err != nil {
			return err
		}
	case UserSubscription:
		command := db.v.B().Hset().Key(key).FieldValue().
			FieldValue("type", sub.Type()).
			FieldValue("version", sub.CurrentVersion()).
			Build()
		err := db.v.Do(context.Background(), command).Error()
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown subscription type")
	}

	// sadd <feedID>:subs <targetID>
	command := db.v.B().Sadd().Key(sanitize(feedID) + ":subs").Member(targetID).Build()
	return db.v.Do(context.Background(), command).Error()
}

func (db DB) GetSubscriptions(feedID string) ([]string, error) {
	// smembers <feedID>:subs
	command := db.v.B().Smembers().Key(sanitize(feedID) + ":subs").Build()
	resp := db.v.Do(context.Background(), command)
	if err := resp.Error(); err != nil {
		return nil, err
	}

	subscriptionIDs, err := resp.AsStrSlice()
	if err != nil {
		return nil, err
	}

	return subscriptionIDs, nil
}

func (db DB) GetSubscription(feedID string, targetID string) (Subscription, error) {
	key := sanitize(feedID) + ":subs:" + targetID

	// hgetall <feedID>:subs:<targetID>
	command := db.v.B().Hgetall().Key(key).Build()
	result, err := db.v.Do(context.Background(), command).AsStrMap()
	if err != nil {
		return nil, err
	}

	if len(result) <= 0 {
		return nil, fmt.Errorf("subscription not found")
	}

	subType := result["type"]
	version := result["version"]

	switch subType {
	case "guild":
		rolesStr := result["roles"]
		var roles []string
		if rolesStr == "" {
			roles = []string{}
		} else {
			roles = strings.Split(rolesStr, ",")
		}
		return GuildSubscription{
			Version: version,
			Roles:   roles,
		}, nil
	case "user":
		return UserSubscription{
			Version: version,
		}, nil
	default:
		return nil, fmt.Errorf("unknown subscription type: %s", subType)
	}
}

func (db DB) RemoveSubscription(feedID string, targetID string) error {
	// del <feedID>:subs:<targetID>
	command := db.v.B().Del().Key(sanitize(feedID) + ":subs:" + targetID).Build()
	if err := db.v.Do(context.Background(), command).Error(); err != nil {
		return err
	}

	// srem <feedID>:subs <targetID>
	command = db.v.B().Srem().Key(sanitize(feedID) + ":subs").Member(targetID).Build()
	return db.v.Do(context.Background(), command).Error()
}

// May return nil!
func (db DB) GetLatestPost(feedID string) ([]byte, error) {
	// get <feedID>:latest
	command := db.v.B().Get().Key(sanitize(feedID) + ":latest").Build()
	resp := db.v.Do(context.Background(), command)
	err := resp.Error()
	if err != nil {
		if valkey.IsValkeyNil(err) {
			return nil, nil
		}
		return nil, err
	}
	raw, err := resp.AsBytes()
	if err != nil {
		return nil, err
	}
	return raw, err
}

func (db DB) SetLatestPost(feedID string, content string) error {
	// set <feedID>:latest <content>
	command := db.v.B().Set().Key(sanitize(feedID) + ":latest").Value(content).Build()
	return db.v.Do(context.Background(), command).Error()
}

type OAuthToken struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
}

// May return nil!
func (db DB) GetOAuthToken() (*OAuthToken, error) {
	// hgetall oauth_token
	command := db.v.B().Hgetall().Key("oauth_token").Build()
	result, err := db.v.Do(context.Background(), command).AsStrMap()
	if err != nil {
		return nil, err
	}
	if len(result) <= 0 {
		return nil, nil
	}

	expiresAt, err := strconv.ParseInt(result["expires_at"], 10, 64)
	if err != nil {
		return nil, err
	}

	return &OAuthToken{
		AccessToken:  result["access_token"],
		RefreshToken: result["refresh_token"],
		ExpiresAt:    time.Unix(expiresAt, 0),
	}, nil
}

func (db DB) SetOAuthToken(oAuthToken OAuthToken) error {
	// hset oauth_token ...
	command := db.v.B().Hset().Key("oauth_token").FieldValue().
		FieldValue("access_token", oAuthToken.AccessToken).
		FieldValue("refresh_token", oAuthToken.RefreshToken).
		FieldValue("expires_at", strconv.FormatInt(oAuthToken.ExpiresAt.Unix(), 10)).
		Build()
	return db.v.Do(context.Background(), command).Error()
}

// May return empty!
func (db DB) GetProfileUUID() (string, error) {
	// get profile_uuid
	command := db.v.B().Get().Key("profile_uuid").Build()
	resp := db.v.Do(context.Background(), command)
	err := resp.Error()
	if err != nil {
		if valkey.IsValkeyNil(err) {
			return "", nil
		}
		return "", err
	}
	raw, err := resp.AsBytes()
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func (db DB) SetProfileUUID(uuid string) error {
	// set profile_uuid <uuid>
	command := db.v.B().Set().Key("profile_uuid").Value(uuid).Build()
	return db.v.Do(context.Background(), command).Error()
}

type GameSessionToken struct {
	SessionToken string
	ExpiresAt    time.Time
}

// May return nil!
func (db DB) GetGameSession() (*GameSessionToken, error) {
	// hgetall game_session
	command := db.v.B().Hgetall().Key("game_session").Build()
	result, err := db.v.Do(context.Background(), command).AsStrMap()
	if err != nil {
		return nil, err
	}
	if len(result) <= 0 {
		return nil, nil
	}

	expiresAt, err := strconv.ParseInt(result["expires_at"], 10, 64)
	if err != nil {
		return nil, err
	}

	return &GameSessionToken{
		SessionToken: result["session_token"],
		ExpiresAt:    time.Unix(expiresAt, 0),
	}, nil
}

func (db DB) SetGameSession(sessionToken GameSessionToken) error {
	// hset game_session ...
	command := db.v.B().Hset().Key("game_session").FieldValue().
		FieldValue("session_token", sessionToken.SessionToken).
		FieldValue("expires_at", strconv.FormatInt(sessionToken.ExpiresAt.Unix(), 10)).
		Build()
	return db.v.Do(context.Background(), command).Error()
}

// May return nil!
func (db DB) GetKratosRefresh() (*time.Time, error) {
	// get kratos_refresh
	command := db.v.B().Get().Key("kratos_refresh").Build()
	resp := db.v.Do(context.Background(), command)
	err := resp.Error()
	if err != nil {
		if valkey.IsValkeyNil(err) {
			return nil, nil
		}
		return nil, err
	}
	raw, err := resp.AsBytes()
	if err != nil {
		return nil, err
	}
	refreshUnix, err := strconv.ParseInt(string(raw), 10, 64)
	if err != nil {
		return nil, err
	}
	refreshTime := time.Unix(refreshUnix, 0)
	return &refreshTime, nil
}

func (db DB) SetKratosRefresh(refresh time.Time) error {
	// set kratos_refresh <refresh>
	command := db.v.B().Set().Key("kratos_refresh").Value(strconv.FormatInt(refresh.Unix(), 10)).Build()
	return db.v.Do(context.Background(), command).Error()
}

func (db DB) Clear() {
	command := db.v.B().Flushdb().Sync().Build()
	db.v.Do(context.Background(), command)
}
