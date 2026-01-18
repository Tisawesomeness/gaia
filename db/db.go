package db

import (
	"context"
	"strconv"
	"time"

	"github.com/Tisawesomeness/gaia/config"
	"github.com/valkey-io/valkey-go"
)

type DB struct {
	v valkey.Client
}

func NewDB(config config.Config) (*DB, error) {
	options := valkey.ClientOption{
		InitAddress: []string{config.Valkey.Address + ":" + strconv.Itoa(config.Valkey.Port)},
	}
	if config.Valkey.Password != "" {
		options.Password = config.Valkey.Password
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

func (db DB) AddOrUpdateSubscription(subType string, channelId string, currentVersion string) error {
	command := db.v.B().Hset().Key(subType+":subs").FieldValue().FieldValue(channelId, currentVersion).Build()
	return db.v.Do(context.Background(), command).Error()
}

// Returns a mapping of channel ID to last notified version
func (db DB) GetSubscriptions(subType string) (map[string]string, error) {
	command := db.v.B().Hgetall().Key(subType + ":subs").Build()
	return db.v.Do(context.Background(), command).AsStrMap()
}

func (db DB) RemoveSubscription(subType string, channelId string) error {
	command := db.v.B().Hdel().Key(subType + ":subs").Field(channelId).Build()
	return db.v.Do(context.Background(), command).Error()
}

// May return nil!
func (db DB) GetLatestPost(subType string) ([]byte, error) {
	command := db.v.B().Get().Key(subType + ":latest").Build()
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

func (db DB) SetLatestPost(subType string, content string) error {
	command := db.v.B().Set().Key(subType + ":latest").Value(content).Build()
	return db.v.Do(context.Background(), command).Error()
}

type OAuthToken struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
}

// May return nil!
func (db DB) GetOAuthToken() (*OAuthToken, error) {
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
	command := db.v.B().Hset().Key("oauth_token").FieldValue().
		FieldValue("access_token", oAuthToken.AccessToken).
		FieldValue("refresh_token", oAuthToken.RefreshToken).
		FieldValue("expires_at", strconv.FormatInt(oAuthToken.ExpiresAt.Unix(), 10)).
		Build()
	return db.v.Do(context.Background(), command).Error()
}

// May return empty!
func (db DB) GetProfileUUID() (string, error) {
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
	command := db.v.B().Set().Key("profile_uuid").Value(uuid).Build()
	return db.v.Do(context.Background(), command).Error()
}

type GameSessionToken struct {
	SessionToken string
	ExpiresAt    time.Time
}

// May return nil!
func (db DB) GetGameSession() (*GameSessionToken, error) {
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
	command := db.v.B().Hset().Key("game_session").FieldValue().
		FieldValue("session_token", sessionToken.SessionToken).
		FieldValue("expires_at", strconv.FormatInt(sessionToken.ExpiresAt.Unix(), 10)).
		Build()
	return db.v.Do(context.Background(), command).Error()
}
