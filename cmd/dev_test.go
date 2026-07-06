package cmd

import (
	"strings"
	"testing"

	"github.com/Tisawesomeness/gaia/testutil/itestutil"
	"github.com/Tisawesomeness/gaia/testutil/testutil"
	"github.com/bwmarrin/discordgo"
	"github.com/stretchr/testify/assert"
)

var (
	devCE       *MockExecutor
	devTestCase = testutil.MakeTestCase(beforeEachDev, nil)
)

func setupDev() {
	ce, err := InitMockExecutor(nil, nil)
	if err != nil {
		panic(err)
	}
	devCE = ce
}

func teardownDev() {
	devCE.DB.Close()
}

func beforeEachDev() {
	devCE.DB.Clear()
}

func assertContentOrFileName(t *testing.T, reply *discordgo.InteractionResponseData, content string) {
	if reply == nil {
		t.Fatal("Reply cannot be nil")
		return
	}
	replyContent := itestutil.ExtractMainContent(reply)
	if strings.Contains(replyContent, content) {
		return
	}
	for _, file := range reply.Files {
		if strings.Contains(file.Name, content) {
			return
		}
	}
	t.Errorf("String `%s` not found in reply content or file name", content)
}

func TestDevCommands(t *testing.T) {
	setupDev()
	t.Cleanup(teardownDev)

	t.Run("/maven", devTestCase(func(t *testing.T) {
		ctx := NewMockContext(devCE)
		ctx.RunCommand("maven")

		assert.Equal(t, 1, len(ctx.replies))
		assertContentOrFileName(t, ctx.replies[0], "xml")
	}))

	t.Run("/maven type=full", devTestCase(func(t *testing.T) {
		ctx := NewMockContext(devCE).WithOptionString("type", "full")
		ctx.RunCommand("maven")

		assert.Equal(t, 1, len(ctx.replies))
		assertContentOrFileName(t, ctx.replies[0], "xml")
	}))

	t.Run("/maven patchline=pre-release", devTestCase(func(t *testing.T) {
		ctx := NewMockContext(devCE).WithOptionString("patchline", "pre-release")
		ctx.RunCommand("maven")

		assert.Equal(t, 1, len(ctx.replies))
		assertContentOrFileName(t, ctx.replies[0], "xml")
	}))

	t.Run("/gradle", devTestCase(func(t *testing.T) {
		ctx := NewMockContext(devCE)
		ctx.RunCommand("gradle")

		assert.Equal(t, 1, len(ctx.replies))
		assertContentOrFileName(t, ctx.replies[0], "kts")
	}))

	t.Run("/gradle type=full", devTestCase(func(t *testing.T) {
		ctx := NewMockContext(devCE).WithOptionString("type", "full")
		ctx.RunCommand("gradle")

		assert.Equal(t, 1, len(ctx.replies))
		assertContentOrFileName(t, ctx.replies[0], "kts")
	}))

	t.Run("/gradle flavor=groovy", devTestCase(func(t *testing.T) {
		ctx := NewMockContext(devCE).WithOptionString("flavor", "groovy")
		ctx.RunCommand("gradle")

		assert.Equal(t, 1, len(ctx.replies))
		assertContentOrFileName(t, ctx.replies[0], "gradle")
	}))

	t.Run("/gradle flavor=groovy type=full", devTestCase(func(t *testing.T) {
		ctx := NewMockContext(devCE).WithOptionString("flavor", "groovy").WithOptionString("type", "full")
		ctx.RunCommand("gradle")

		assert.Equal(t, 1, len(ctx.replies))
		assertContentOrFileName(t, ctx.replies[0], "gradle")
	}))
}
