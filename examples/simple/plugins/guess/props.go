/*
Package guess provides example code to setup stateful command.

This command returns sarah.UserContext as part of sarah.CommandResponse until user inputs correct number.
As long as sarah.UserContext is returned, the next input from the same user is fed to the function defined in sarah.UserContext.
When user guesses right number or input .abort, the context is removed and user is free to input next desired command.

This example uses in-memory storage to store user context.
See https://github.com/oklahomer/go-sarah-rediscontext to use external storage.
*/
package guess

import (
	"context"
	"github.com/oklahomer/go-sarah/v4"
	"github.com/oklahomer/go-sarah/v4/slack"
	"math/rand"
	"strconv"
	"strings"
	"time"
)

func init() {
	sarah.RegisterCommandProps(SlackProps)
}

// SlackProps is a pre-built guess command properties for Slack.
var SlackProps = sarah.NewCommandPropsBuilder().
	BotType(slack.SLACK).
	Identifier("guess").
	Instruction("Input .guess to start a game.").
	MatchFunc(func(input sarah.Input) bool {
		return strings.HasPrefix(strings.TrimSpace(input.Message()), ".guess")
	}).
	Func(func(ctx context.Context, input sarah.Input) (*sarah.CommandResponse, error) {
		// Generate answer value at the very beginning.
		rand.Seed(time.Now().UnixNano())
		answer := rand.Intn(10)

		// Let user guess the right answer.
		return slack.NewResponse(
			input,
			"Input number.",
			slack.RespWithNext(func(c context.Context, i sarah.Input) (*sarah.CommandResponse, error) {
				return guessFunc(c, i, answer)
			}),
			slack.RespAsThreadReply(true),
		)
	}).
	MustBuild()

func guessFunc(_ context.Context, input sarah.Input, answer int) (*sarah.CommandResponse, error) {
	// For handiness, create a function that recursively calls guessFunc until user input right answer.
	retry := func(c context.Context, i sarah.Input) (*sarah.CommandResponse, error) {
		return guessFunc(c, i, answer)
	}

	// See if user inputs valid number.
	guess, err := strconv.Atoi(strings.TrimSpace(input.Message()))
	if err != nil {
		return slack.NewResponse(input, "Invalid input format.", slack.RespWithNext(retry), slack.RespAsThreadReply(true))
	}

	// If guess is right, tell user and finish current user context.
	// Otherwise let user input next guess with bit of a hint.
	if guess == answer {
		return slack.NewResponse(input, "Correct!", slack.RespAsThreadReply(true))
	} else if guess > answer {
		return slack.NewResponse(input, "Smaller!", slack.RespWithNext(retry), slack.RespAsThreadReply(true))
	} else {
		return slack.NewResponse(input, "Bigger!", slack.RespWithNext(retry), slack.RespAsThreadReply(true))
	}
}
