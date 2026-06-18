package cmd

import (
	"fmt"
	"log"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/Tisawesomeness/gaia/auth"
	"github.com/Tisawesomeness/gaia/config"
	"github.com/Tisawesomeness/gaia/db"
	"github.com/Tisawesomeness/gaia/hytale"
	"github.com/bwmarrin/discordgo"
)

type OptionsMap map[string]*discordgo.ApplicationCommandInteractionDataOption

type CustomID struct {
	InteractionType string
	Action          string
	SessionID       string
}

func (c CustomID) String() string {
	return fmt.Sprintf("%s_%s_%s", c.InteractionType, c.Action, c.SessionID)
}

func (c CustomID) WithAction(action string) CustomID {
	return CustomID{
		InteractionType: c.InteractionType,
		Action:          action,
		SessionID:       c.SessionID,
	}
}

func parseCustomID(customID string) *CustomID {
	arr := strings.Split(customID, "_")
	if len(arr) != 3 {
		return nil
	}
	return &CustomID{
		InteractionType: arr[0],
		Action:          arr[1],
		SessionID:       arr[2],
	}
}

// Contains utility methods that make commands less error-prone:
//
// - Reply... methods with mentions disabled by default and work whether deferred or not
//
// - User() gets the user without having to nil-check Interaction.User / Interaction.Member
//
// - etc.
type CommandContext interface {
	Config() *config.Config
	DB() *db.DB
	HTTP() *http.Client
	AuthStore() auth.AuthStore
	HytaleFeeds() *hytale.HytaleFeeds
	Breakers() *Breakers
	BotMetadata() *BotMetadata
	// The raw Discord session, prefer using CommandContext methods since this makes the command untestable
	Session() *discordgo.Session
	// The raw Discord event, prefer using CommandContext methods since this makes the command untestable
	Interaction() *discordgo.InteractionCreate

	// Generates a map of option ID to option data
	Options() OptionsMap
	// Gets the user that executed this command, works in both guilds and DMs
	User() *discordgo.User
	// Checks if the user has permission and is in the right context to execute the provided command
	UserCanExecute(command *discordgo.ApplicationCommand) bool
	// The ID of the guild the command was executed in. Empty if not executed in a guild.
	GuildID() string
	// Fetches the channels of the provided guild.
	GuildChannels(guildID string) ([]*discordgo.Channel, error)

	// A unique ID for each interaction with the bot (ex: slash command invocation, button press)
	InteractionID() string
	// The ID of the component clicked. Empty if no component was clicked.
	CustomID() *CustomID
	// Tracks a new interaction with the provided ID and state. Overwrites old interactions.
	NewInteraction(id string, state any)

	// Signals to Discord that the command was received and a follow-up will come later.
	// After deferring, regular replies (InteractionResponseChannelMessageWithSource) will do nothing.
	// Use the ctx.Reply... methods instead.
	DeferReply()
	Reply(content string)
	ReplyEphemeral(content string)
	ReplyEmbed(embed *discordgo.MessageEmbed)
	ReplyComplex(data *discordgo.InteractionResponseData)
	ReplyWarn(content string)
	ReplyExternalError(content string)
	ReplyError(message string, err error)
	Edit(data *discordgo.InteractionResponseData)
}

type commandContext struct {
	config      *config.Config
	db          *db.DB
	http        *http.Client
	authStore   auth.AuthStore
	hytaleFeeds *hytale.HytaleFeeds
	breakers    *Breakers
	botMetadata *BotMetadata

	session     *discordgo.Session
	interaction *discordgo.InteractionCreate
	hasDeferred bool
}

func (ce CommandExecutor) newCommandContext(s *discordgo.Session, i *discordgo.InteractionCreate) *commandContext {
	return &commandContext{
		config:      ce.Config,
		db:          ce.DB,
		http:        ce.HTTP,
		authStore:   ce.AuthStore,
		hytaleFeeds: ce.HytaleFeeds,
		breakers:    ce.Breakers,
		botMetadata: ce.BotMetadata,
		session:     s,
		interaction: i,
		hasDeferred: false,
	}
}

func (ctx *commandContext) Config() *config.Config                    { return ctx.config }
func (ctx *commandContext) DB() *db.DB                                { return ctx.db }
func (ctx *commandContext) HTTP() *http.Client                        { return ctx.http }
func (ctx *commandContext) AuthStore() auth.AuthStore                 { return ctx.authStore }
func (ctx *commandContext) HytaleFeeds() *hytale.HytaleFeeds          { return ctx.hytaleFeeds }
func (ctx *commandContext) Breakers() *Breakers                       { return ctx.breakers }
func (ctx *commandContext) BotMetadata() *BotMetadata                 { return ctx.botMetadata }
func (ctx *commandContext) Session() *discordgo.Session               { return ctx.session }
func (ctx *commandContext) Interaction() *discordgo.InteractionCreate { return ctx.interaction }

func (ctx *commandContext) Options() OptionsMap {
	options := ctx.Interaction().ApplicationCommandData().Options
	optionMap := make(OptionsMap, len(options))
	for _, opt := range options {
		optionMap[opt.Name] = opt
	}
	return optionMap
}

func (ctx *commandContext) User() *discordgo.User {
	if ctx.Interaction().Member != nil {
		return ctx.Interaction().Member.User
	} else {
		return ctx.Interaction().User
	}
}

func (ctx *commandContext) UserCanExecute(command *discordgo.ApplicationCommand) bool {
	if command.Contexts != nil && !slices.Contains(*command.Contexts, ctx.Interaction().Context) {
		return false
	}
	if command.DefaultMemberPermissions == nil {
		return true
	}
	if ctx.Interaction().Member != nil {
		return (ctx.Interaction().Member.Permissions & *command.DefaultMemberPermissions) == *command.DefaultMemberPermissions
	} else {
		return true
	}
}

func (ctx *commandContext) GuildID() string {
	return ctx.Interaction().GuildID
}

func (ctx *commandContext) GuildChannels(guildID string) ([]*discordgo.Channel, error) {
	return ctx.Session().GuildChannels(guildID)
}

func (ctx *commandContext) InteractionID() string {
	return ctx.Interaction().Interaction.ID
}

func (ctx *commandContext) CustomID() *CustomID {
	if ctx.Interaction().Type != discordgo.InteractionMessageComponent {
		return nil
	}
	return parseCustomID(ctx.Interaction().MessageComponentData().CustomID)
}

func (ctx *commandContext) NewInteraction(id string, state any) {
	interactionSessions[id] = &InteractionSession{
		State:    state,
		UserID:   ctx.User().ID,
		LastUsed: time.Now(),
	}
}

func (ctx *commandContext) DeferReply() {
	ctx.hasDeferred = true
	ctx.Session().InteractionRespond(ctx.Interaction().Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})
}

func (ctx *commandContext) Reply(content string) {
	ctx.ReplyComplex(&discordgo.InteractionResponseData{
		Content: content,
	})
}

func (ctx *commandContext) ReplyEphemeral(content string) {
	ctx.ReplyComplex(&discordgo.InteractionResponseData{
		Content: content,
		Flags:   discordgo.MessageFlagsEphemeral,
	})
}

func (ctx *commandContext) ReplyEmbed(embed *discordgo.MessageEmbed) {
	ctx.ReplyComplex(&discordgo.InteractionResponseData{
		Embeds: []*discordgo.MessageEmbed{embed},
	})
}

func (ctx *commandContext) ReplyWarn(content string) {
	ctx.ReplyEphemeral(":warning: " + content)
}

func (ctx *commandContext) ReplyExternalError(content string) {
	ctx.ReplyEphemeral(":x: " + content)
}

func (ctx *commandContext) ReplyError(message string, err error) {
	ctx.ReplyEphemeral(":boom: " + message)
	if err != nil {
		ctx.reportError(err)
	}
}

func (ctx *commandContext) ReplyComplex(data *discordgo.InteractionResponseData) {
	// Default to no mentions allowed
	if data.AllowedMentions == nil {
		data.AllowedMentions = &discordgo.MessageAllowedMentions{}
	}

	for _, embed := range data.Embeds {
		if embed.Footer == nil {
			embed.Footer = &discordgo.MessageEmbedFooter{
				Text: "Gaia " + ctx.BotMetadata().Version,
			}
		}
	}

	var err error
	if ctx.hasDeferred {
		var attachments []*discordgo.MessageAttachment
		if data.Attachments == nil {
			attachments = []*discordgo.MessageAttachment{}
		} else {
			attachments = *data.Attachments
		}

		_, err = ctx.Session().FollowupMessageCreate(ctx.Interaction().Interaction, false, &discordgo.WebhookParams{
			Content:         data.Content,
			Components:      data.Components,
			Embeds:          data.Embeds,
			TTS:             data.TTS,
			Files:           data.Files,
			Attachments:     attachments,
			AllowedMentions: data.AllowedMentions,
			Flags:           data.Flags,
		})
	} else {
		err = ctx.Session().InteractionRespond(ctx.Interaction().Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: data,
		})
	}

	if err != nil {
		ctx.reportError(err)
	}
}

func (ctx *commandContext) Edit(data *discordgo.InteractionResponseData) {
	ctx.Session().InteractionRespond(ctx.Interaction().Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: data,
	})
}

func (ctx *commandContext) reportError(err error) {
	id := ctx.Interaction().ApplicationCommandData().Name
	options := formatCommandOptions(ctx.Interaction().ApplicationCommandData().Options)
	log.Printf("Error in command /%s options %s:\n%+v", id, options, err)
}
