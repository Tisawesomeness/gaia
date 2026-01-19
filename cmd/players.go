package cmd

import (
	"fmt"
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
				Name:        "identifier",
				Description: "The UUID or username of the profile to fetch",
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

func profileCommand(s *discordgo.Session, i *discordgo.InteractionCreate, ctx *CommandContext) {
	options := i.ApplicationCommandData().Options
	identifier := options[0].Value.(string)

	var profile hytale.PublicGameProfile
	var err error
	uuid, isUUID := validateAndFormatUUID(identifier)
	if isUUID {
		profile, err = hytale.FetchProfileFromUUID(uuid, ctx.Config, ctx.HTTP, ctx.AuthStore)
	} else if usernameRegex.MatchString(identifier) {
		profile, err = hytale.FetchProfileFromUsername(identifier, ctx.Config, ctx.HTTP, ctx.AuthStore)
	} else {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: identifier + " is not a valid username or UUID",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	if err != nil {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Failed to fetch profile: " + err.Error(),
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	embed := &discordgo.MessageEmbed{
		Title: "Profile for " + profile.Username,
		Description: fmt.Sprintf("Short UUID: `%s`\nLong UUID: `%s`",
			strings.ReplaceAll(profile.UUUID, "-", ""),
			profile.UUUID),
		Color: 0x00FF00,
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

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
		},
	})
}
