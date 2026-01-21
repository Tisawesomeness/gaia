package cmd

import (
	"fmt"
	"log"
	"regexp"
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
		Description: "Fetch a Hytale player's skin details by UUID or username",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "player",
				Description: "Username or UUID",
				Required:    true,
			},
		},
	}

	uuidRegex     = regexp.MustCompile(`^([0-9a-fA-F]{8})-?([0-9a-fA-F]{4})-?([0-9a-fA-F]{4})-?([0-9a-fA-F]{4})-?([0-9a-fA-F]{12})$`)
	usernameRegex = regexp.MustCompile(`^[a-zA-Z0-9_]{3,16}$`)

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
					display: "Overpants",
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

// Checks if the provided UUID is valid and formats it with dashes if necessary.
func validateAndFormatUUID(uuid string) (string, bool) {
	matches := uuidRegex.FindStringSubmatch(uuid)
	if matches == nil {
		return "", false
	}
	return fmt.Sprintf("%s-%s-%s-%s-%s", matches[1], matches[2], matches[3], matches[4], matches[5]), true
}

func tryFetchProfileFromUUID(uuid string, ctx *CommandContext) (*hytale.PublicGameProfile, error) {
	return util.Execute(ctx.Breakers.HytaleSession, func() (*hytale.PublicGameProfile, error) {
		return hytale.FetchProfileFromUUID(uuid, ctx.Config, ctx.HTTP, ctx.AuthStore)
	})
}

func tryFetchProfileFromUsername(username string, ctx *CommandContext) (*hytale.PublicGameProfile, error) {
	return util.Execute(ctx.Breakers.HytaleSession, func() (*hytale.PublicGameProfile, error) {
		return hytale.FetchProfileFromUsername(username, ctx.Config, ctx.HTTP, ctx.AuthStore)
	})
}

func profileCommand(ctx *CommandContext) {
	identifier := ctx.Options()["player"].StringValue()

	var profile *hytale.PublicGameProfile
	var err error
	uuid, isUUID := validateAndFormatUUID(identifier)
	if isUUID {
		ctx.DeferReply()
		profile, err = tryFetchProfileFromUUID(uuid, ctx)
	} else if usernameRegex.MatchString(identifier) {
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
			checkAvailability(identifier, ctx)
			return
		}
	}

	ctx.ReplyEmbed(&discordgo.MessageEmbed{
		Title: "Profile for " + profile.Username,
		Description: fmt.Sprintf("Short UUID: `%s`\nLong UUID: `%s`",
			strings.ReplaceAll(profile.UUID, "-", ""),
			profile.UUID),
		Color: 0x00FF00,
	})
}

func checkAvailability(username string, ctx *CommandContext) {
	_, err := ctx.Breakers.KratosSession.Execute(func() (any, error) {
		availability, err := hytale.CheckAvailability(username, ctx.Config, ctx.HTTP)
		if err != nil {
			return nil, err
		}

		switch availability {
		case hytale.Available:
			ctx.ReplyEmbed(&discordgo.MessageEmbed{
				Title:       "Profile for " + username,
				Description: "Username available",
				Color:       0xFFFFFF,
			})
		case hytale.Reserved:
			ctx.ReplyEmbed(&discordgo.MessageEmbed{
				Title:       "Profile for " + username,
				Description: "Username reserved",
				Color:       0xFFFF00,
			})
		case hytale.HytaleReserved:
			ctx.ReplyEmbed(&discordgo.MessageEmbed{
				Title:       "Profile for " + username,
				Description: "Username reserved by the Hytale Team",
				Color:       0x00FFFF,
			})
		case hytale.Prohibited:
			ctx.ReplyEmbed(&discordgo.MessageEmbed{
				Title:       "Profile for " + username,
				Description: "Username contains a prohibited word",
				Color:       0x00FFFF,
			})
		case hytale.InUse:
			// If profile returns 404 but username is in use,
			// either Hytale is lying, or we got unlucky with timing
			log.Printf("Username %s in use, but profile returned 404!", username)
			ctx.ReplyExternalError("An error occurred while contacting Hytale servers.")
		case hytale.Unknown:
			ctx.ReplyEmbed(&discordgo.MessageEmbed{
				Title:       "Profile for " + username,
				Description: "Username not in use (unknown status)",
				Color:       0x000000,
			})
		}

		return nil, nil
	})

	if err != nil {
		log.Printf("Error checking availability (circuit breaker): %v", err)
		ctx.ReplyEmbed(&discordgo.MessageEmbed{
			Title:       "Profile for " + username,
			Description: "Username not in use (unknown status)",
			Color:       0x000000,
		})
	}
}

func skinCommand(ctx *CommandContext) {
	identifier := ctx.Options()["player"].StringValue()

	var profile *hytale.PublicGameProfile
	var err error
	uuid, isUUID := validateAndFormatUUID(identifier)
	if isUUID {
		ctx.DeferReply()
		profile, err = tryFetchProfileFromUUID(uuid, ctx)
	} else if usernameRegex.MatchString(identifier) {
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
		Title: "Skin Details for " + profile.Username,
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
