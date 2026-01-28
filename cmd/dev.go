package cmd

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"text/template"

	"github.com/Tisawesomeness/gaia/hytale"
	"github.com/bwmarrin/discordgo"
)

var (
	MavenCommand = &discordgo.ApplicationCommand{
		Name:        "maven",
		Description: "Generate a maven build file for Hytale development",
		Options: []*discordgo.ApplicationCommandOption{
			patchlineOption,
		},
	}

	GradleCommand = &discordgo.ApplicationCommand{
		Name:        "gradle",
		Description: "Generate a gradle build file for Hytale development",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "flavor",
				Description: "The language the gradle script should be in",
				Choices: []*discordgo.ApplicationCommandOptionChoice{
					{
						Name:  "Kotlin (build.gradle.kts)",
						Value: "kotlin",
					},
					{
						Name:  "Groovy (build.gradle)",
						Value: "groovy",
					},
				},
			},
			patchlineOption,
		},
	}
)

var (
	//go:embed templates/pom.xml
	mavenTemplateStr string
	mavenTemplate    = template.Must(template.New("maven").Parse(mavenTemplateStr))
	//go:embed templates/build.gradle
	gradleTemplateStr string
	gradleTemplate    = template.Must(template.New("gradle").Parse(gradleTemplateStr))
	//go:embed templates/build.gradle.kts
	gradleKtsTemplateStr string
	gradleKtsTemplate    = template.Must(template.New("gradleKts").Parse(gradleKtsTemplateStr))
)

type variables struct {
	Repository string
	Group      string
	Artifact   string
	Version    string
}

func mavenCommand(ctx *CommandContext) {
	buildScriptCommand(ctx, mavenTemplate, "pom.xml", "xml")
}

func gradleCommand(ctx *CommandContext) {
	options := ctx.Options()

	flavor := "kotlin"
	if option, exists := options["flavor"]; exists {
		flavor = option.StringValue()
	}

	if flavor == "kotlin" {
		buildScriptCommand(ctx, gradleKtsTemplate, "build.gradle.kits", "kts")
	} else {
		buildScriptCommand(ctx, gradleTemplate, "build.gradle", "gradle")
	}
}

func buildScriptCommand(ctx *CommandContext, template *template.Template, fileName string, lang string) {
	options := ctx.Options()

	patchlineValue := "release"
	if option, exists := options["patchline"]; exists {
		patchlineValue = option.StringValue()
	}

	patchline, err := hytale.ParsePatchline(patchlineValue)
	if err != nil {
		ctx.ReplyWarn("Invalid patchline")
		return
	}

	feed, exists := ctx.HytaleFeeds.Feeds[hytale.GetFeedType(patchline, hytale.Server)]
	if !exists {
		ctx.ReplyError(errors.New("Could not retrieve the latest Hytale version."))
		return
	}

	version := feed.GetVersion()
	if version == "" {
		ctx.ReplyError(errors.New("Could not retrieve the latest Hytale version."))
		return
	}

	vars := variables{
		Repository: ctx.Config.Feeds.MavenRepo,
		Group:      ctx.Config.Feeds.MavenGroup,
		Artifact:   ctx.Config.Feeds.MavenArtifact,
		Version:    version,
	}
	var buf bytes.Buffer
	err = template.Execute(&buf, vars)
	if err != nil {
		ctx.ReplyError(errors.New("Could not generate build file"))
		return
	}

	if fileName == "pom.xml" {
		ctx.ReplyComplex(&discordgo.InteractionResponseData{
			Files: []*discordgo.File{
				{
					Name:   fileName,
					Reader: &buf,
				},
			},
		})
	} else {
		ctx.Reply(fmt.Sprintf("```%s\n%s\n```", lang, buf.String()))
	}
}
