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
	"github.com/oklahomer/go-sarah"
	"github.com/oklahomer/go-sarah/slack"
	"golang.org/x/net/context"
	"math/rand"
	"strconv"
	"strings"
	"time"
)

var SlackProps = sarah.NewCommandPropsBuilder().
	BotType(slack.SLACK).
	Identifier("guess").
	InputExample(".guess").
	MatchFunc(func(input sarah.Input) bool {
		return strings.HasPrefix(strings.TrimSpace(input.Message()), ".guess")
	}).
	Func(func(ctx context.Context, input sarah.Input) (*sarah.CommandResponse, error) {
		// Generate answer value at the very beginning.
		rand.Seed(time.Now().UnixNano())
		answer := rand.Intn(10)

		// Let user guess the right answer.
		return slack.NewStringResponseWithNext("Input number.", func(c context.Context, i sarah.Input) (*sarah.CommandResponse, error) {
			return guessFunc(c, i, answer)
		}), nil
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
		return slack.NewStringResponseWithNext("Invalid input format.", retry), nil
	}

	// If guess is right, tell user and finish current user context.
	// Otherwise let user input next guess with bit of a hint.
	if guess == answer {
		return slack.NewStringResponse("Correct!"), nil
	} else if guess > answer {
		return slack.NewStringResponseWithNext("Smaller!", retry), nil
	} else {
		return slack.NewStringResponseWithNext("Bigger!", retry), nil
	}
}
