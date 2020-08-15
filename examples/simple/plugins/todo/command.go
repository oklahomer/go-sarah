/*
Package todo is an example of stateful command that let users input required arguments step by step in a conversational manner.

On each valid input, given argument is stashed to *args.
*args is passed around until all required arguments are filled.
*/
package todo

import (
	"context"
	"fmt"
	"github.com/oklahomer/go-sarah/v3"
	"github.com/oklahomer/go-sarah/v3/log"
	"github.com/oklahomer/go-sarah/v3/slack"
	"regexp"
	"strings"
	"time"
)

var matchPattern = regexp.MustCompile(`^\.todo`)

// DummyStorage is an empty struct that represents a permanent storage.
type DummyStorage struct {
}

// Save saves given todo settings to permanent storage.
func (s *DummyStorage) Save(senderKey string, description string, due time.Time) {
	// Write to storage
}

type args struct {
	description string
	due         time.Time
}

// BuildCommand builds todo command with given storage.
func BuildCommand(storage *DummyStorage) sarah.Command {
	return &command{
		storage: storage,
	}
}

type command struct {
	storage *DummyStorage
}

var _ sarah.Command = (*command)(nil)

func (cmd *command) Identifier() string {
	return "todo"
}

func (cmd *command) Execute(_ context.Context, input sarah.Input) (*sarah.CommandResponse, error) {
	stripped := sarah.StripMessage(matchPattern, input.Message())
	if stripped == "" {
		// If description is not given, let user proceed to input one.
		return slack.NewResponse(input, "Please input a thing to do.", slack.RespWithNext(cmd.inputDesc))
	}

	args := &args{
		description: stripped,
	}
	next := func(c context.Context, i sarah.Input) (*sarah.CommandResponse, error) {
		return cmd.inputDate(c, i, args)
	}

	return slack.NewResponse(input, "Please input the due date in YYYY-MM-DD format.", slack.RespWithNext(next))
}

func (cmd *command) Instruction(_ *sarah.HelpInput) string {
	return `Input ".todo buy milk" to add "buy milk" to your TODO list.`
}

func (cmd *command) Match(input sarah.Input) bool {
	return strings.HasPrefix(strings.TrimSpace(input.Message()), ".todo")
}

func (cmd *command) inputDesc(_ context.Context, input sarah.Input) (*sarah.CommandResponse, error) {
	description := strings.TrimSpace(input.Message())
	if description == "" {
		// If no description is provided, let user input.
		next := func(c context.Context, i sarah.Input) (*sarah.CommandResponse, error) {
			return cmd.inputDesc(c, i)
		}
		return slack.NewResponse(input, "Please input a thing to do.", slack.RespWithNext(next))
	}

	// Let user proceed to next step to input due date.
	next := func(c context.Context, i sarah.Input) (*sarah.CommandResponse, error) {
		args := &args{
			description: description,
		}
		return cmd.inputDate(c, i, args)
	}
	return slack.NewResponse(input, "Input the due date in YYYY-MM-DD format.", slack.RespWithNext(next))
}

func (cmd *command) inputDate(_ context.Context, input sarah.Input, args *args) (*sarah.CommandResponse, error) {
	date := strings.TrimSpace(input.Message())

	reinput := func(c context.Context, i sarah.Input) (*sarah.CommandResponse, error) {
		return cmd.inputDate(c, i, args)
	}
	if date == "" {
		// If no due date is provided, let user input.
		return slack.NewResponse(input, "Please input the due date in YYYY-MM-DD format.", slack.RespWithNext(reinput))
	}

	_, err := time.Parse("2006-01-02", date)
	if err != nil {
		return slack.NewResponse(input, "Please input valid date in YYYY-MM-DD format.", slack.RespWithNext(reinput))
	}

	next := func(c context.Context, i sarah.Input) (*sarah.CommandResponse, error) {
		return cmd.inputTime(c, i, date, args)
	}
	return slack.NewResponse(input, "Input the due time in HH:MM format. N if not specified.", slack.RespWithNext(next))
}

func (cmd *command) inputTime(_ context.Context, input sarah.Input, validDate string, args *args) (*sarah.CommandResponse, error) {
	t := strings.TrimSpace(input.Message())

	reinput := func(c context.Context, i sarah.Input) (*sarah.CommandResponse, error) {
		return cmd.inputTime(c, i, validDate, args)
	}
	if t == "" {
		return slack.NewResponse(input, "Please input the due time in HH:MM format.")
	}

	if strings.ToLower(t) == "n" {
		// If there is no due time, consider the last minute is the due time.
		t = "23:59"
	}

	_, err := time.Parse("15:04", t)
	if err != nil {
		return slack.NewResponse(input, "Please input a valid due time in HH:MM format.", slack.RespWithNext(reinput))
	}

	due, err := time.Parse("2006-01-02 15:04", fmt.Sprintf("%s %s", validDate, t))
	if err != nil {
		// Should not reach here since previous time parse succeeded.
		log.Error("Failed to parse due date: %+v", err)
		return slack.NewResponse(input, "Fatal error occurred")
	}

	args.due = due
	next := func(c context.Context, i sarah.Input) (*sarah.CommandResponse, error) {
		return cmd.confirm(c, i, args)
	}
	confirmMessage := fmt.Sprintf("TODO: %s. Due is %s\nIs this O.K.? Y/N", args.description, args.due.Format("2006-01-02 15:04"))
	return slack.NewResponse(input, confirmMessage, slack.RespWithNext(next))
}

func (cmd *command) confirm(_ context.Context, input sarah.Input, args *args) (*sarah.CommandResponse, error) {
	msg := strings.TrimSpace(input.Message())
	if msg != "" {
		msg = strings.ToLower(msg)
	}

	if msg == "y" {
		cmd.storage.Save(input.SenderKey(), args.description, args.due)
		return slack.NewResponse(input, "Saved.")
	}

	if msg == "n" {
		return slack.NewResponse(input, "Aborted.")
	}

	reinput := func(c context.Context, i sarah.Input) (*sarah.CommandResponse, error) {
		return cmd.confirm(c, i, args)
	}
	return slack.NewResponse(input, "Please input Y or N.", slack.RespWithNext(reinput))
}
