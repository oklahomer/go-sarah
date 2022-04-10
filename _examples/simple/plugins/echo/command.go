// Package echo provides an example of sarah.Command implementation.
//
// The use of sarah.CommandProps is a better way to provide a set of command properties to Sarah
// especially when a configuration file must be supervised and the configuration values need to be updated on file update.
// When such a configuration supervision is not required, a developer may implement sarah.Command interface herself
// and feed its instance to Sarah via sarah.RegisterCommand or to sarah.Bot via its AppendCommand method.
package echo

import (
	"context"
	"github.com/oklahomer/go-sarah/v4"
	"github.com/oklahomer/go-sarah/v4/slack"
	"regexp"
)

var matchPattern = regexp.MustCompile(`^\.echo`)

type command struct {
}

// Identifier returns command ID.
func (c *command) Identifier() string {
	return "echo"
}

// Execute receives a user input and returns a result of this Command execution.
func (c *command) Execute(_ context.Context, input sarah.Input) (*sarah.CommandResponse, error) {
	return slack.NewResponse(input, sarah.StripMessage(matchPattern, input.Message()))
}

// Instruction provides a guide for the requesting user.
func (c *command) Instruction(_ *sarah.HelpInput) string {
	return ".echo foo"
}

// Match checks if the user input matches and this Command must be executed.
func (c *command) Match(input sarah.Input) bool {
	return matchPattern.MatchString(input.Message())
}

// Command is a command instance that can directly fed to sarah.RegisterCommand or Bot.AppendCommand.
var Command = &command{}
