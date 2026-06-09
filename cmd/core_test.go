package cmd

import (
	"testing"

	"github.com/Tisawesomeness/gaia/testutil"
	"github.com/stretchr/testify/assert"
)

var (
	coreCE       *CommandExecutor
	coreTestCase = testutil.MakeTestCase(beforeEachCore, nil)
)

func init() {
	ce, err := InitMockExecutor()
	if err != nil {
		panic(err)
	}
	coreCE = ce
}

func teardownCore() {
	coreCE.DB.Close()
}

func beforeEachCore() {
	coreCE.DB.ClearAll()
}

func TestCoreCommands(t *testing.T) {
	t.Cleanup(teardownCore)

	t.Run("/help", coreTestCase(func(t *testing.T) {
		ctx := NewMockContext(coreCE, nil)
		helpCommand(ctx)
		// Simply test that the bot sends one reply
		assert.Equal(t, 1, len(ctx.replies))
	}))

	// /info not testable since it uses ctx.Session()

	t.Run("/credits", coreTestCase(func(t *testing.T) {
		ctx := NewMockContext(coreCE, nil)
		creditsCommand(ctx)
		// Simply test that the bot sends one reply
		assert.Equal(t, 1, len(ctx.replies))
	}))
}
