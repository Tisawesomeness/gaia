package cmd

import (
	"fmt"
	"log"
	"strings"

	"github.com/Tisawesomeness/gaia/hytale"
	"github.com/Tisawesomeness/gaia/util"
	"github.com/bwmarrin/discordgo"
)

var (
	ProfileCommand = &discordgo.ApplicationCommand{
		Name:        "profile",
		Description: "Fetch a Hytale profile by UUID or username",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "player",
				Description: "Username or UUID",
				Required:    true,
			},
		},
	}

	SkinCommand = &discordgo.ApplicationCommand{
		Name:        "skin",
		Description: "Fetch a Hytale player's skin details",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "player",
				Description: "Username or UUID",
				Required:    true,
			},
		},
	}
)

func NewRenderCommand(name string, description string, renderType hytale.RenderType) *Command {
	return &Command{
		discord: &discordgo.ApplicationCommand{
			Name:        name,
			Description: description,
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "player",
					Description: "Username or UUID",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionInteger,
					Name:        "size",
					Description: "Image Size",
					Choices: []*discordgo.ApplicationCommandOptionChoice{
						{
							Name:  "64x64",
							Value: 64,
						},
						{
							Name:  "128x128",
							Value: 128,
						},
						{
							Name:  "256x256",
							Value: 256,
						},
						{
							Name:  "512x512",
							Value: 512,
						},
						{
							Name:  "1024x1024",
							Value: 1024,
						},
						{
							Name:  "2048x2048",
							Value: 2048,
						},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionInteger,
					Name:        "rotate",
					Description: "Player rotation",
					MinValue:    &minDegrees,
					MaxValue:    maxDegrees,
				},
			},
		},
		handler: func(ctx CommandContext) {
			renderCommand(ctx, renderType)
		},
	}
}

var (
	minDegrees = -360.0
	maxDegrees = 360.0

	cosmeticGroups = []cosmeticGroup{
		{
			name: "Head",
			cosmetics: []cosmeticType{
				{
					display: "Haircut",
					id:      "haircut",
				},
				{
					display: "Eyebrows",
					id:      "eyebrows",
				},
				{
					display: "Eyes",
					id:      "eyes",
				},
				{
					display: "Facial Hair",
					id:      "facialHair",
				},
				{
					display: "Head Accessory",
					id:      "headAccessory",
				},
				{
					display: "Face Accessory",
					id:      "faceAccessory",
				},
				{
					display: "Ear Accessory",
					id:      "earAccessory",
				},
			},
		},
		{
			name: "General",
			cosmetics: []cosmeticType{
				{
					display: "Underwear",
					id:      "underwear",
				},
				{
					display: "Body Characteristic",
					id:      "bodyCharacteristic",
				},
				{
					display: "Face",
					id:      "face",
				},
				{
					display: "Mouth",
					id:      "mouth",
				},
				{
					display: "Ears",
					id:      "ears",
				},
			},
		},
		{
			name: "Torso",
			cosmetics: []cosmeticType{
				{
					display: "Undertop",
					id:      "undertop",
				},
				{
					display: "Overtop",
					id:      "overtop",
				},
				{
					display: "Gloves",
					id:      "gloves",
				},
			},
		},
		{
			name: "Legs",
			cosmetics: []cosmeticType{
				{
					display: "Pants",
					id:      "pants",
				},
				{
					display: "Legwear",
					id:      "overpants",
				},
				{
					display: "Shoes",
					id:      "shoes",
				},
			},
		},
		{
			name: "Extra",
			cosmetics: []cosmeticType{
				{
					display: "Skin Feature",
					id:      "skinFeature",
				},
				{
					display: "Cape",
					id:      "cape",
				},
			},
		},
	}

	knownCosmeticIDs = buildKnownCosmeticIDs()
)

func buildKnownCosmeticIDs() map[string]struct{} {
	knownCosmeticIDs := make(map[string]struct{})
	for _, group := range cosmeticGroups {
		for _, cosmetic := range group.cosmetics {
			knownCosmeticIDs[cosmetic.id] = struct{}{}
		}
	}
	return knownCosmeticIDs
}

type cosmeticType struct {
	display string
	id      string
}

type cosmeticGroup struct {
	name      string
	cosmetics []cosmeticType
}

func tryFetchProfileFromUUID(uuid string, ctx CommandContext) (*hytale.PublicGameProfile, error) {
	return util.Execute(ctx.Breakers().HytaleSession, func() (*hytale.PublicGameProfile, error) {
		return hytale.FetchProfileFromUUID(ctx.Config(), ctx.HTTP(), ctx.AuthStore(), uuid)
	})
}

func tryFetchProfileFromUsername(username string, ctx CommandContext) (*hytale.PublicGameProfile, error) {
	return util.Execute(ctx.Breakers().HytaleSession, func() (*hytale.PublicGameProfile, error) {
		return hytale.FetchProfileFromUsername(ctx.Config(), ctx.HTTP(), ctx.AuthStore(), username)
	})
}

func profileCommand(ctx CommandContext) {
	identifier := ctx.Options()["player"].StringValue()

	var profile *hytale.PublicGameProfile
	var err error
	uuid, isUUID := hytale.ValidateUUID(identifier)
	if isUUID {
		ctx.DeferReply()
		profile, err = tryFetchProfileFromUUID(uuid, ctx)
	} else if hytale.ValidateUsername(identifier) {
		ctx.DeferReply()
		profile, err = tryFetchProfileFromUsername(identifier, ctx)
	} else {
		ctx.ReplyWarn(fmt.Sprintf("`%s` is not a valid username or UUID", identifier))
		return
	}

	if err != nil {
		log.Printf("Could not fetch profile %s: %v", identifier, err)
		ctx.ReplyExternalError("An error occurred while contacting Hytale servers.")
		return
	}
	if profile == nil {
		if isUUID {
			ctx.Reply(fmt.Sprintf("There is no player with the UUID `%s`.", identifier))
			return
		} else {
			ctx.Reply(fmt.Sprintf("There is no player with the username `%s`.", identifier))
			return
		}
	}

	ctx.ReplyEmbed(&discordgo.MessageEmbed{
		Author: &discordgo.MessageEmbedAuthor{
			Name:    "Profile for " + profile.Username,
			URL:     ctx.Config().Profile.ProfileWebsite + profile.Username,
			IconURL: hytale.RenderHead(ctx.Config(), profile.Username, 128, 30),
		},
		Thumbnail: &discordgo.MessageEmbedThumbnail{
			URL: hytale.RenderFullBody(ctx.Config(), profile.Username, 512, 360-30),
		},
		Description: fmt.Sprintf("Short UUID: `%s`\nLong UUID: `%s`",
			strings.ReplaceAll(profile.UUID, "-", ""),
			profile.UUID),
		Color: 0x00FF00,
	})
}

func skinCommand(ctx CommandContext) {
	identifier := ctx.Options()["player"].StringValue()

	var profile *hytale.PublicGameProfile
	var err error
	uuid, isUUID := hytale.ValidateUUID(identifier)
	if isUUID {
		ctx.DeferReply()
		profile, err = tryFetchProfileFromUUID(uuid, ctx)
	} else if hytale.ValidateUsername(identifier) {
		ctx.DeferReply()
		profile, err = tryFetchProfileFromUsername(identifier, ctx)
	} else {
		ctx.ReplyWarn(fmt.Sprintf("`%s` is not a valid username or UUID", identifier))
		return
	}

	if err != nil {
		log.Printf("Could not fetch profile %s: %v", identifier, err)
		ctx.ReplyExternalError("An error occurred while contacting Hytale servers.")
		return
	}
	if profile == nil {
		if isUUID {
			ctx.Reply(fmt.Sprintf("There is no player with the UUID `%s`.", identifier))
		} else {
			ctx.Reply(fmt.Sprintf("There is no player with the username `%s`.", identifier))
		}
		return
	}

	embed := &discordgo.MessageEmbed{
		Author: &discordgo.MessageEmbedAuthor{
			Name:    "Skin details for " + profile.Username,
			URL:     ctx.Config().Profile.ProfileWebsite + profile.Username,
			IconURL: hytale.RenderHead(ctx.Config(), profile.Username, 128, 30),
		},
		Image: &discordgo.MessageEmbedImage{
			URL: hytale.RenderFullBody(ctx.Config(), profile.Username, 2048, 0),
		},
		Color: 0x00FF00,
	}

	if len(profile.Skin) <= 0 {
		embed.Description = "(no skin)"
		ctx.ReplyEmbed(embed)
		return
	}

	for _, group := range cosmeticGroups {
		fieldValue := ""
		for _, cosmetic := range group.cosmetics {
			if value, exists := profile.Skin[cosmetic.id]; exists {
				if value != nil {
					fieldValue += fmt.Sprintf("%s: `%s`\n", cosmetic.display, *value)
				} else {
					fieldValue += fmt.Sprintf("%s: (none)\n", cosmetic.display)
				}
			}
		}
		// There may be new cosmetic types added in the future
		// Add them to the end of the Extra group
		if group.name == "Extra" {
			for cosmeticID, value := range profile.Skin {
				if _, exists := knownCosmeticIDs[cosmeticID]; !exists {
					displayName := util.ToCapitalizedSpacedWords(cosmeticID)
					if value != nil {
						fieldValue += fmt.Sprintf("%s: `%s`\n", displayName, *value)
					} else {
						fieldValue += fmt.Sprintf("%s: (none)\n", displayName)
					}
				}
			}
		}
		if fieldValue != "" {
			embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
				Name:   group.name,
				Value:  fieldValue,
				Inline: false,
			})
		}
	}

	ctx.ReplyEmbed(embed)
}

func renderCommand(ctx CommandContext, renderType hytale.RenderType) {
	options := ctx.Options()
	identifier := options["player"].StringValue()

	var size int
	sizeOption, exists := options["size"]
	if exists {
		size = int(sizeOption.IntValue())
	} else {
		size = 512
	}

	var rotate int
	rotateOption, exists := options["rotate"]
	if exists {
		rotate = util.WrapDegrees(int(rotateOption.IntValue()))
	} else {
		rotate = 0
	}

	var profile *hytale.PublicGameProfile
	var err error
	uuid, isUUID := hytale.ValidateUUID(identifier)
	if isUUID {
		ctx.DeferReply()
		profile, err = tryFetchProfileFromUUID(uuid, ctx)
	} else if hytale.ValidateUsername(identifier) {
		ctx.DeferReply()
		profile, err = tryFetchProfileFromUsername(identifier, ctx)
	} else {
		ctx.ReplyWarn(fmt.Sprintf("`%s` is not a valid username or UUID", identifier))
		return
	}

	if err != nil {
		log.Printf("Could not fetch profile %s: %v", identifier, err)
		ctx.ReplyExternalError("An error occurred while contacting Hytale servers.")
		return
	}

	cape, hasCape := profile.Skin["cape"]
	if (!hasCape || cape == nil) && renderType == hytale.CapeRender {
		ctx.Reply("That player does not have a cape.")
		return
	}

	embed := &discordgo.MessageEmbed{
		Image: &discordgo.MessageEmbedImage{
			URL: renderType.Render(ctx.Config(), profile.Username, size, rotate),
		},
	}

	ctx.ReplyEmbed(embed)
}
