package hello

import (
	"github.com/oklahomer/go-sarah"
	"github.com/oklahomer/go-sarah/slack"
	"golang.org/x/net/context"
	"regexp"
)

// Command provides default setup of random user command.
// If different setup with another identifier, match pattern, etc. directly feed CommandFunc to preferred CommandBuilder
var Command = sarah.NewCommandBuilder().
	Identifier("hello").
	InputExample(".hello").
	MatchPattern(regexp.MustCompile(`\.hello`)).
	Func(func(_ context.Context, input sarah.Input) (*sarah.CommandResponse, error) {
		return slack.NewStringResponse("Hello!"), nil
	}).
	MustBuild()
