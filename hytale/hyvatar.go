package hytale

import (
	"fmt"
	"net/url"

	"github.com/Tisawesomeness/gaia/config"
)

type RenderType int

const (
	HeadRender RenderType = iota
	FullBodyRender
	CapeRender
)

func (rt RenderType) Render(config *config.Config, username string, size int, rotate int) string {
	switch rt {
	case HeadRender:
		return RenderHead(config, username, size, rotate)
	case FullBodyRender:
		return RenderFullBody(config, username, size, rotate)
	case CapeRender:
		return RenderCape(config, username, size, rotate)
	default:
		panic("Unknown render type")
	}
}

func RenderHead(config *config.Config, username string, size int, rotate int) string {
	return renderURL(username, size, rotate, config.Profile.Hyvatar)
}
func RenderFullBody(config *config.Config, username string, size int, rotate int) string {
	return renderURL(username, size, rotate, config.Profile.Hyvatar+"full/")
}
func RenderCape(config *config.Config, username string, size int, rotate int) string {
	return renderURL(username, size, rotate, config.Profile.Hyvatar+"cape/")
}

func renderURL(username string, size int, rotate int, baseUrl string) string {
	forUsername := baseUrl + username

	params := url.Values{}
	if size != 128 {
		params.Set("size", fmt.Sprintf("%d", size))
	}
	if rotate != 0 {
		params.Set("rotate", fmt.Sprintf("%d", rotate))
	}

	if len(params) > 0 {
		return fmt.Sprintf("%s?%s", forUsername, params.Encode())
	} else {
		return forUsername
	}
}
