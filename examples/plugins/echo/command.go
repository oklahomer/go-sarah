/*
Package echo provides example code to sarah.Command implementation.

CommandProps is a better way to provide set of command properties to Runner
especially when configuration file must be supervised and configuration struct needs to be updated on file update;
Developer may implement Command interface herself and feed its instance to Bot via Bot.AppendCommand
when command specification is simple.
*/
package echo

import (
	"github.com/oklahomer/go-sarah"
	"github.com/oklahomer/go-sarah/slack"
	"golang.org/x/net/context"
	"regexp"
)

var matchPattern = regexp.MustCompile(`^\.echo`)

type command struct {
}

// Identifier returns command ID.
func (c *command) Identifier() string {
	return "echo"
}

// Execute receives user input and returns results of this Command.
func (c *command) Execute(_ context.Context, input sarah.Input) (*sarah.CommandResponse, error) {
	return slack.NewStringResponse(sarah.StripMessage(matchPattern, input.Message())), nil
}

// InputExample provides input example for user.
func (c *command) InputExample() string {
	return ".echo foo"
}

// Match checks if user input matches to this Command.
func (c *command) Match(input sarah.Input) bool {
	return matchPattern.MatchString(input.Message())
}

// Command is a command instance that can directly fed to Bot.AppendCommand.
var Command = &command{}
