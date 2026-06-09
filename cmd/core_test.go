package cmd

import (
	"testing"

	"github.com/Tisawesomeness/gaia/testutil"
	"github.com/stretchr/testify/assert"
)

var (
	testCE   *CommandExecutor
	testCase = testutil.MakeTestCase(beforeEach, nil)
)

func init() {
	ce, err := InitMockExecutor()
	if err != nil {
		panic(err)
	}
	testCE = ce
}

func teardown() {
	testCE.DB.Close()
}

func beforeEach() {
	testCE.DB.ClearAll()
}

func TestCommands(t *testing.T) {
	t.Cleanup(teardown)

	t.Run("/credits", testCase(func(t *testing.T) {
		ctx := NewMockContext(testCE, nil)
		creditsCommand(ctx)
		assert.Equal(t, 1, len(ctx.replies))
	}))
}
