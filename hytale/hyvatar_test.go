package hytale

import (
	"net/url"
	"testing"

	"github.com/Tisawesomeness/gaia/config"
	"github.com/stretchr/testify/assert"
)

func TestRenderURLs(t *testing.T) {
	config := &config.Config{
		Profile: config.ProfileConfig{
			Hyvatar: "https://hyvatar.example.com/",
		},
	}

	username := "testuser"

	t.Run("head render defaults", func(t *testing.T) {
		got := RenderHead(config, username, 128, 0)
		want := "https://hyvatar.example.com/testuser"
		assert.Equal(t, want, got)
	})

	t.Run("head render with size", func(t *testing.T) {
		got, err := url.Parse(RenderHead(config, username, 256, 0))
		assert.NoError(t, err)
		assert.Equal(t, "/testuser", got.Path)
		params := got.Query()
		assert.Equal(t, "256", params.Get("size"))
		assert.False(t, params.Has("rotate"))
	})

	t.Run("head render with rotate", func(t *testing.T) {
		got, err := url.Parse(RenderHead(config, username, 128, 90))
		assert.NoError(t, err)
		assert.Equal(t, "/testuser", got.Path)
		params := got.Query()
		assert.Equal(t, "90", params.Get("rotate"))
		assert.False(t, params.Has("size"))
	})

	t.Run("head render with size and rotate", func(t *testing.T) {
		got, err := url.Parse(RenderHead(config, username, 256, 90))
		assert.NoError(t, err)
		assert.Equal(t, "/testuser", got.Path)
		params := got.Query()
		assert.Equal(t, "256", params.Get("size"))
		assert.Equal(t, "90", params.Get("rotate"))
	})

	t.Run("full body render defaults", func(t *testing.T) {
		got := RenderFullBody(config, username, 128, 0)
		want := "https://hyvatar.example.com/full/testuser"
		assert.Equal(t, want, got)
	})

	t.Run("full body render with size", func(t *testing.T) {
		got, err := url.Parse(RenderFullBody(config, username, 512, 0))
		assert.NoError(t, err)
		want, _ := url.Parse("https://hyvatar.example.com/full/testuser?size=512")
		assert.Equal(t, want.String(), got.String())
		assert.Equal(t, "/full/testuser", got.Path)
		params := got.Query()
		assert.Equal(t, "512", params.Get("size"))
		assert.False(t, params.Has("rotate"))
	})

	t.Run("full body render with rotate", func(t *testing.T) {
		got, err := url.Parse(RenderFullBody(config, username, 128, 180))
		assert.NoError(t, err)
		want, _ := url.Parse("https://hyvatar.example.com/full/testuser?rotate=180")
		assert.Equal(t, want.String(), got.String())
		assert.Equal(t, "/full/testuser", got.Path)
		params := got.Query()
		assert.Equal(t, "180", params.Get("rotate"))
		assert.False(t, params.Has("size"))
	})

	t.Run("cape render defaults", func(t *testing.T) {
		got := RenderCape(config, username, 128, 0)
		want := "https://hyvatar.example.com/cape/testuser"
		assert.Equal(t, want, got)
	})

	t.Run("cape render with size", func(t *testing.T) {
		got, err := url.Parse(RenderCape(config, username, 256, 0))
		assert.NoError(t, err)
		assert.Equal(t, "/cape/testuser", got.Path)
		params := got.Query()
		assert.Equal(t, "256", params.Get("size"))
		assert.False(t, params.Has("rotate"))
	})

	t.Run("cape render with rotate", func(t *testing.T) {
		got, err := url.Parse(RenderCape(config, username, 128, 270))
		assert.NoError(t, err)
		assert.Equal(t, "/cape/testuser", got.Path)
		params := got.Query()
		assert.Equal(t, "270", params.Get("rotate"))
		assert.False(t, params.Has("size"))
	})

	t.Run("cape render with size and rotate", func(t *testing.T) {
		got, err := url.Parse(RenderCape(config, username, 256, 180))
		assert.NoError(t, err)
		assert.Equal(t, "/cape/testuser", got.Path)
		params := got.Query()
		assert.Equal(t, "256", params.Get("size"))
		assert.Equal(t, "180", params.Get("rotate"))
	})
}
