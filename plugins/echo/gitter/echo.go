package gitter

import (
	"github.com/oklahomer/go-sarah"
	"github.com/oklahomer/go-sarah/gitter"
	"golang.org/x/net/context"
	"regexp"
)

var (
	identifier = "echo"
)

func echo(_ context.Context, strippedMessage string, _ sarah.BotInput, _ sarah.CommandConfig) (*sarah.PluginResponse, error) {
	return gitter.NewStringResponse(strippedMessage), nil
}

func init() {
	builder := sarah.NewCommandBuilder().
		Identifier(identifier).
		ConfigStruct(sarah.NullConfig).
		MatchPattern(regexp.MustCompile(`^\.echo`)).
		Func(echo).
		Example(".echo knock knock")
	sarah.AppendCommandBuilder(gitter.GITTER, builder)
}
