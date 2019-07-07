[![GoDoc](https://godoc.org/github.com/oklahomer/go-sarah?status.svg)](https://godoc.org/github.com/oklahomer/go-sarah)
[![Go Report Card](https://goreportcard.com/badge/github.com/oklahomer/go-sarah)](https://goreportcard.com/report/github.com/oklahomer/go-sarah)
[![Build Status](https://travis-ci.org/oklahomer/go-sarah.svg?branch=master)](https://travis-ci.org/oklahomer/go-sarah)
[![Coverage Status](https://coveralls.io/repos/github/oklahomer/go-sarah/badge.svg?branch=master)](https://coveralls.io/github/oklahomer/go-sarah?branch=master)
[![Maintainability](https://api.codeclimate.com/v1/badges/a2f0df359bec1552b28f/maintainability)](https://codeclimate.com/github/oklahomer/go-sarah/maintainability) [![Join the chat at https://gitter.im/go-sarah-dev/community](https://badges.gitter.im/go-sarah-dev/community.svg)](https://gitter.im/go-sarah-dev/community?utm_source=badge&utm_medium=badge&utm_campaign=pr-badge&utm_content=badge)

# Introduction
Sarah is a general purpose bot framework named after author's firstborn daughter.

While the first goal is to prep author to write Go-ish code, the second goal is to provide simple yet highly customizable bot framework.

# Supported Chat Services/Protocols
Although a developer may implement `sarah.Adapter` to integrate with a desired chat service,
some adapters are provided as reference implementations:
- [Slack](https://github.com/oklahomer/go-sarah/tree/master/slack)
- [Gitter](https://github.com/oklahomer/go-sarah/tree/master/gitter)
- [XMPP](https://github.com/oklahomer/go-sarah-xmpp)
- [LINE](https://github.com/oklahomer/go-sarah-line)

# At a Glance
![hello world](/doc/img/hello.png)

Above is a general use of `go-sarah`.
Registered commands are checked against user input and matching one is executed;
when a user inputs ".hello," hello command is executed and a message "Hello, 世界" is returned.

Below image depicts how a command with user's **conversational context** works.
The idea and implementation of "user's conversational context" is go-sarah's signature feature that makes bot command "**state-aware**."

![](/doc/img/todo_captioned.png)

Above example is a good way to let user input series of arguments in a conversational manner.
Below is another example that use stateful command to entertain user.

![](/doc/img/guess_captioned.png)

Following is the minimal code that implements such general command and stateful command introduced above.
In this example, two ways to implement [`sarah.Command`](https://github.com/oklahomer/go-sarah/wiki/Command) are shown.
One simply implements `sarah.Command` interface; while another uses `sarah.CommandPropsBuilder` for lazy construction.
Detailed benefits of using `sarah.CommandPropsBuilder` and `sarah.CommandProps` are described at its wiki page, [CommandPropsBuilder](https://github.com/oklahomer/go-sarah/wiki/CommandPropsBuilder).

For more practical examples, see [./examples](https://github.com/oklahomer/go-sarah/tree/master/examples).

```go
package main

import (
	"context"
	"fmt"
	"github.com/oklahomer/go-sarah"
	"github.com/oklahomer/go-sarah/slack"
	
	// Below packages register commands in their init().
	// Importing with blank identifier will do the magic.
	_ "guess"
	_ "hello"
)

func main() {
	// Setup slack adapter.
	slackConfig := slack.NewConfig()
	slackConfig.Token = "REPLACE THIS"
	adapter, err := slack.NewAdapter(slackConfig)
	if err != nil {
		panic(fmt.Errorf("faileld to setup Slack Adapter: %s", err.Error()))
	}

	// Setup storage.
	cacheConfig := sarah.NewCacheConfig()
	storage := sarah.NewUserContextStorage(cacheConfig)

	// Setup Bot with slack adapter and default storage.
	bot, err := sarah.NewBot(adapter, sarah.BotWithStorage(storage))
	if err != nil {
		panic(fmt.Errorf("faileld to setup Slack Bot: %s", err.Error()))
	}
	sarah.RegisterBot(bot)

	// Run
	config := sarah.NewConfig()
	err = sarah.Run(context.TODO(), config)
	if err != nil {
		panic(fmt.Errorf("failed to run: %s", err.Error()))
	}
}
```
```go
package guess

import (
	"context"
	"github.com/oklahomer/go-sarah"
	"github.com/oklahomer/go-sarah/slack"
	"math/rand"
	"strconv"
	"strings"
	"time"
)

func init() {
	sarah.RegisterCommandProps(props)
}

var props = sarah.NewCommandPropsBuilder().
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
		return slack.NewResponse(input, "Input number.", slack.RespWithNext(func(c context.Context, i sarah.Input) (*sarah.CommandResponse, error){
			return guessFunc(c, i, answer)
		}))
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
		return slack.NewResponse(input, "Invalid input format.", slack.RespWithNext(retry))
	}

	// If guess is right, tell user and finish current user context.
	// Otherwise let user input next guess with bit of a hint.
	if guess == answer {
		return slack.NewResponse(input, "Correct!")
	} else if guess > answer {
		return slack.NewResponse(input, "Smaller!", slack.RespWithNext(retry))
	} else {
		return slack.NewResponse(input, "Bigger!", slack.RespWithNext(retry))
	}
}
```

```go
package hello

import (
	"context"
	"github.com/oklahomer/go-sarah"
	"github.com/oklahomer/go-sarah/slack"
	"strings"
)

func init() {
    sarah.RegisterCommand(slack.SLACK, &command{})	
}

type command struct {
}

var _ sarah.Command = (*command)(nil)

func (hello *command) Identifier() string {
	return "hello"
}

func (hello *command) Execute(_ context.Context, i sarah.Input) (*sarah.CommandResponse, error) {
	return slack.NewResponse(i, "Hello!")
}

func (hello *command) Instruction(input *sarah.HelpInput) string {
	if 12 < input.SentAt().Hour() {
		// This command is only active in the morning.
		// Do not show instruction in the afternoon.
		return ""
	}
	return "Input .hello to greet"
}

func (hello *command) Match(input sarah.Input) bool {
	return strings.TrimSpace(input.Message()) == ".hello"
}

```

# Overview
`go-sarah` is a general purpose bot framework that enables developers to create and customize their own bot experiences with any chat service.
This comes with a unique feature called "_**stateful command**_" as well as some basic features such as _**command**_ and _**scheduled task**_.
In addition to those features, this provides rich life cycle management including _**live configuration update**_, _**customizable alerting mechanism**_, _**automated command/task (re-)building**_ and _**concurrent command/task execution**_.

`go-sarah` is composed of fine grained components to provide above features.
Those components have their own interfaces and default implementations, so developers are free to customize bot behavior by supplying own implementation.

![component diagram](/doc/uml/components.png)

Follow below links for details:
- [Project wiki](https://github.com/oklahomer/go-sarah/wiki)
- [GoDoc](https://godoc.org/github.com/oklahomer/go-sarah)
- [Example codes](https://github.com/oklahomer/go-sarah/tree/master/examples)
