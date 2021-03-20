/*
Package echo provides example code to sarah.Command implementation.

CommandProps is a better way to provide set of command properties to Runner
especially when configuration file must be supervised and configuration struct needs to be updated on file update;
Developer may implement Command interface herself and feed its instance to Bot via Bot.AppendCommand
when command specification is simple.
*/
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

// Execute receives user input and returns results of this Command.
func (c *command) Execute(_ context.Context, input sarah.Input) (*sarah.CommandResponse, error) {
	return slack.NewResponse(input, sarah.StripMessage(matchPattern, input.Message()))
}

// Instruction provides input instruction for user.
func (c *command) Instruction(_ *sarah.HelpInput) string {
	return ".echo foo"
}

// Match checks if user input matches to this Command.
func (c *command) Match(input sarah.Input) bool {
	// Once Runner receives input from Bot, it dispatches task to worker where multiple tasks may run in concurrent manner.
	// Searching for corresponding Command is an important part of this task, which means Command.Match is called simultaneously from multiple goroutines.
	// To avoid lock contention, Command developer should consider copying the *regexp.Regexp object.
	return matchPattern.Copy().MatchString(input.Message())
}

// Command is a command instance that can directly fed to Bot.AppendCommand.
var Command = &command{}
