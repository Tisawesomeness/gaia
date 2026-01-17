package db

import (
	"context"
	"strconv"

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
