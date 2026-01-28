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
	scriptSizeOption = &discordgo.ApplicationCommandOption{
		Type:        discordgo.ApplicationCommandOptionString,
		Name:        "type",
		Description: "Whether to show the full script or only the dependency",
		Choices: []*discordgo.ApplicationCommandOptionChoice{
			{
				Name:  "Dependency",
				Value: "dependency",
			},
			{
				Name:  "Full",
				Value: "full",
			},
		},
	}

	MavenCommand = &discordgo.ApplicationCommand{
		Name:        "maven",
		Description: "Generate a maven build file for Hytale development",
		Options: []*discordgo.ApplicationCommandOption{
			scriptSizeOption,
			patchlineOption,
		},
	}

	GradleCommand = &discordgo.ApplicationCommand{
		Name:        "gradle",
		Description: "Generate a gradle build file for Hytale development",
		Options: []*discordgo.ApplicationCommandOption{
			scriptSizeOption,
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

type scriptType int

const (
	maven scriptType = iota
	gradle
	gradleKts
)

func (s scriptType) fileName() string {
	switch s {
	case maven:
		return "pom.xml"
	case gradle:
		return "build.gradle"
	case gradleKts:
		return "build.gradle.kts"
	default:
		panic("unknown script type")
	}
}

func (s scriptType) display() string {
	switch s {
	case maven:
		return "Maven"
	case gradle:
		return "Groovy Gradle"
	case gradleKts:
		return "Kotlin Gradle"
	default:
		panic("unknown script type")
	}
}

func (s scriptType) codeblockLang() string {
	switch s {
	case maven:
		return "xml"
	case gradle:
		return "gradle"
	case gradleKts:
		return "kts"
	default:
		panic("unknown script type")
	}
}

type buildScript struct {
	template   *template.Template
	scriptType scriptType
	isFull     bool
}

var (
	//go:embed templates/maven_dependency.txt
	mavenDepTemplateStr string
	mavenDep            = buildScript{
		template:   template.Must(template.New("mavenDep").Parse(mavenDepTemplateStr)),
		scriptType: maven,
		isFull:     false,
	}
	//go:embed templates/pom.xml
	mavenFullTemplateStr string
	mavenFull            = buildScript{
		template:   template.Must(template.New("mavenFull").Parse(mavenFullTemplateStr)),
		scriptType: maven,
		isFull:     true,
	}

	//go:embed templates/gradle_dependency.txt
	gradleDepTemplateStr string
	gradleDep            = buildScript{
		template:   template.Must(template.New("gradleDep").Parse(gradleDepTemplateStr)),
		scriptType: gradle,
		isFull:     false,
	}
	//go:embed templates/build.gradle
	gradleFullTemplateStr string
	gradleFull            = buildScript{
		template:   template.Must(template.New("gradleFull").Parse(gradleFullTemplateStr)),
		scriptType: gradle,
		isFull:     true,
	}

	//go:embed templates/gradle_kts_dependency.txt
	gradleKtsDepTemplateStr string
	gradleKtsDep            = buildScript{
		template:   template.Must(template.New("gradleKtsDep").Parse(gradleKtsDepTemplateStr)),
		scriptType: gradleKts,
		isFull:     false,
	}
	//go:embed templates/build.gradle.kts
	gradleKtsFullTemplateStr string
	gradleKtsFull            = buildScript{
		template:   template.Must(template.New("gradleKtsFull").Parse(gradleKtsFullTemplateStr)),
		scriptType: gradleKts,
		isFull:     true,
	}
)

type variables struct {
	Repository string
	Group      string
	Artifact   string
	Version    string
}

func mavenCommand(ctx *CommandContext) {
	options := ctx.Options()

	scriptSize := "dependency"
	if option, exists := options["type"]; exists {
		scriptSize = option.StringValue()
	}

	if scriptSize == "dependency" {
		buildScriptCommand(ctx, mavenDep)
	} else {
		buildScriptCommand(ctx, mavenFull)
	}
}

func gradleCommand(ctx *CommandContext) {
	options := ctx.Options()

	scriptSize := "dependency"
	if option, exists := options["type"]; exists {
		scriptSize = option.StringValue()
	}

	flavor := "kotlin"
	if option, exists := options["flavor"]; exists {
		flavor = option.StringValue()
	}

	if scriptSize == "dependency" {
		if flavor == "kotlin" {
			buildScriptCommand(ctx, gradleKtsDep)
		} else {
			buildScriptCommand(ctx, gradleDep)
		}
	} else {
		if flavor == "kotlin" {
			buildScriptCommand(ctx, gradleKtsFull)
		} else {
			buildScriptCommand(ctx, gradleFull)
		}
	}
}

func buildScriptCommand(ctx *CommandContext, buildScript buildScript) {
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
	err = buildScript.template.Execute(&buf, vars)
	if err != nil {
		ctx.ReplyError(errors.New("Could not generate build file"))
		return
	}

	if !buildScript.isFull {
		ctx.ReplyEmbed(&discordgo.MessageEmbed{
			Title:       fmt.Sprintf("%s Hytale Dependency", buildScript.scriptType.display()),
			Description: fmt.Sprintf("```%s\n%s\n```", buildScript.scriptType.codeblockLang(), buf.String()),
		})
	} else if buildScript.scriptType == maven {
		ctx.ReplyComplex(&discordgo.InteractionResponseData{
			Files: []*discordgo.File{
				{
					Name:   buildScript.scriptType.fileName(),
					Reader: &buf,
				},
			},
		})
	} else {
		ctx.Reply(fmt.Sprintf("```%s\n%s\n```", buildScript.scriptType.codeblockLang(), buf.String()))
	}
}
