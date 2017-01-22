package echo

import (
	"github.com/oklahomer/go-sarah"
	"golang.org/x/net/context"
	"regexp"
)

var (
	identifier   = "echo"
	matchPattern = regexp.MustCompile(`^\.echo`)
)

func CommandFunc(_ context.Context, input sarah.Input) (*sarah.CommandResponse, error) {
	return sarah.NewStringResponse(sarah.StripMessage(matchPattern, input.Message())), nil
}

var SlackCommand = sarah.NewCommandBuilder().
	Identifier(identifier).
	MatchPattern(matchPattern).
	Func(CommandFunc).
	InputExample(".echo knock knock").
	MustBuild()

var GitterCommand = sarah.NewCommandBuilder().
	Identifier(identifier).
	MatchPattern(matchPattern).
	Func(CommandFunc).
	InputExample(".echo knock knock")
