package cmd

import (
	"testing"

	"github.com/Tisawesomeness/gaia/testutil/testutil"
	"github.com/stretchr/testify/assert"
)

var (
	coreCE       *MockExecutor
	coreTestCase = testutil.MakeTestCase(beforeEachCore, nil)
)

func setupCore() {
	ce, err := InitMockExecutor(nil, nil)
	if err != nil {
		panic(err)
	}
	coreCE = ce
}

func teardownCore() {
	coreCE.DB.Close()
}

func beforeEachCore() {
	coreCE.DB.Clear()
}

func TestCoreCommands(t *testing.T) {
	setupCore()
	t.Cleanup(teardownCore)

	t.Run("/help", coreTestCase(func(t *testing.T) {
		ctx := NewMockContext(coreCE)
		ctx.RunCommand("help")
		// Simply test that the bot sends one reply
		assert.Equal(t, 1, len(ctx.replies))
	}))

	// /info not testable since it uses ctx.Session()

	t.Run("/credits", coreTestCase(func(t *testing.T) {
		ctx := NewMockContext(coreCE)
		ctx.RunCommand("credits")
		// Simply test that the bot sends one reply
		assert.Equal(t, 1, len(ctx.replies))
	}))
}
