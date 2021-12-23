// Package guess provides an example to set up a stateful command.
//
// This command returns sarah.UserContext as part of sarah.CommandResponse until the user inputs a correct number.
// As long as sarah.UserContext is returned, the next input from the same user is fed to the function defined in sarah.UserContext.
// When the user guesses the right number or inputs .abort, the user context is removed and the user is free to input the next desired command.
//
// This example uses an in-memory storage to store user contexts.
// See https://github.com/oklahomer/go-sarah-rediscontext to use external storage.
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
		// Generate an answer value at the very beginning.
		rand.Seed(time.Now().UnixNano())
		answer := rand.Intn(10)

		// Let the user guess the right answer.
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
	// For handiness, create a function that recursively calls guessFunc until the user inputs the right answer.
	retry := func(c context.Context, i sarah.Input) (*sarah.CommandResponse, error) {
		return guessFunc(c, i, answer)
	}

	// See if the user inputs the valid number.
	guess, err := strconv.Atoi(strings.TrimSpace(input.Message()))
	if err != nil {
		return slack.NewResponse(input, "Invalid input format.", slack.RespWithNext(retry), slack.RespAsThreadReply(true))
	}

	// If the guess is right, tell the user and finish the current user context.
	// Otherwise, let the user input the next guess with a bit of a hint.
	if guess == answer {
		return slack.NewResponse(input, "Correct!", slack.RespAsThreadReply(true))
	} else if guess > answer {
		return slack.NewResponse(input, "Smaller!", slack.RespWithNext(retry), slack.RespAsThreadReply(true))
	} else {
		return slack.NewResponse(input, "Bigger!", slack.RespWithNext(retry), slack.RespAsThreadReply(true))
	}
}
