[![Go Reference](https://pkg.go.dev/badge/github.com/oklahomer/go-sarah/v4.svg)](https://pkg.go.dev/github.com/oklahomer/go-sarah/v4)
[![Go Report Card](https://goreportcard.com/badge/github.com/oklahomer/go-sarah)](https://goreportcard.com/report/github.com/oklahomer/go-sarah)
[![Build Status](https://travis-ci.com/oklahomer/go-sarah.svg?branch=master)](https://travis-ci.com/oklahomer/go-sarah)
[![Coverage Status](https://coveralls.io/repos/github/oklahomer/go-sarah/badge.svg?branch=master)](https://coveralls.io/github/oklahomer/go-sarah?branch=master)
[![Maintainability](https://api.codeclimate.com/v1/badges/a2f0df359bec1552b28f/maintainability)](https://codeclimate.com/github/oklahomer/go-sarah/maintainability)
[![Join the chat at https://gitter.im/go-sarah-dev/community](https://badges.gitter.im/go-sarah-dev/community.svg)](https://gitter.im/go-sarah-dev/community?utm_source=badge&utm_medium=badge&utm_campaign=pr-badge&utm_content=badge)

# Introduction
Sarah is a general-purpose bot framework named after the author's firstborn daughter.

This comes with a unique feature called "stateful command" as well as some basic features such as command and scheduled task.
In addition to those fundamental features, this project provides rich life cycle management including _**[live configuration update](https://github.com/oklahomer/go-sarah/wiki/Live-Configuration-Update)**_, _**[customizable alerting mechanism](https://github.com/oklahomer/go-sarah/wiki/Alerter)**_, _**automated [command](https://github.com/oklahomer/go-sarah/wiki/CommandPropsBuilder)\/[task](https://github.com/oklahomer/go-sarah/wiki/ScheduledTaskPropsBuilder) (re-)building**_, and _**[panic-proofed concurrent command/task execution](https://github.com/oklahomer/go-sarah/wiki/Worker)**_.

Such features are achieved with a composition of fine-grained components.
Each component has its own interface and a default implementation, so developers are free to customize their bot experience by replacing the default implementation for a particular component with their own implementation.
Thanks to such segmentalized lifecycle management architecture, the [adapter component](https://github.com/oklahomer/go-sarah/wiki/Default-Bot-and-Adapter) to interact with each chat service has fewer responsibilities comparing to other bot frameworks;
An adapter developer may focus on implementing the protocol to interacting with the corresponding chat service.
To take a look at those components and their relations, see [Components](https://github.com/oklahomer/go-sarah/wiki/Components).

# IMPORTANT NOTICE
## v4 Release
This is the fourth major version of `go-sarah`, which involves some architectural changes:
- `sarah.NewBot` now returns a single value: `sarah.Bot`
- Utility packages including logger, retry, and worker are now hosted at `github.com/oklahomer/go-kasumi`

## v3 Release
This is the third major version of `go-sarah`, which introduces the Slack adapter's improvement to support both RTM and Events API.
Breaking interface change for Slack adapter was inevitable and that is the sole reason for this major version up.
Other than that, this does not include any breaking change.
See [Migrating from v2.x to v3.x](https://github.com/oklahomer/go-sarah/wiki/Migrating-from-v2.x-to-v3.x) for details.

## v2 Release
The second major version introduced some breaking changes to `go-sarah`.
This version still supports and maintains all functionalities, better interfaces for easier integration are added.
See [Migrating from v1.x to v2.x](https://github.com/oklahomer/go-sarah/wiki/Migrating-from-v1.x-to-v2.x) to migrate from the older version.

# Supported Chat Services/Protocols
Although a developer may implement `sarah.Adapter` to integrate with the desired chat service,
some adapters are provided as reference implementations:
- [Slack](https://github.com/oklahomer/go-sarah/tree/master/slack)
- [Gitter](https://github.com/oklahomer/go-sarah/tree/master/gitter)
- [XMPP](https://github.com/oklahomer/go-sarah-xmpp)
- [LINE](https://github.com/oklahomer/go-sarah-line)

# At a Glance
## General Command Execution
![hello world](/doc/img/hello.png)

Above is a general use of `go-sarah`.
Registered commands are checked against user input and matching one is executed;
when a user inputs ".hello," hello command is executed and a message "Hello, 世界" is returned.

## Stateful Command Execution
The below image depicts how a command with a user's **conversational context** works.
The idea and implementation of "user's conversational context" is `go-sarah`'s signature feature that makes bot command "**state-aware**."

![](/doc/img/todo_captioned.png)

The above example is a good way to let a user input a series of arguments in a conversational manner.
Below is another example that uses a stateful command to entertain the user.

![](/doc/img/guess_captioned.png)

## Example Code
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
	"github.com/oklahomer/go-sarah/v4"
	"github.com/oklahomer/go-sarah/v4/slack"
	
	"os"
	"os/signal"
	"syscall"
	
	// Below packages register commands in their init().
	// Importing with blank identifier will do the magic.
	_ "guess"
	_ "hello"
)

func main() {
	// Setup Slack adapter
	setupSlack()
	
	// Prepare go-sarah's core context.
	ctx, cancel := context.WithCancel(context.Background())

	// Run
	config := sarah.NewConfig()
	err := sarah.Run(ctx, config)
	if err != nil {
		panic(fmt.Errorf("failed to run: %s", err.Error()))
	}
	
	// Stop when signal is sent.
	c := make(chan os.Signal, 1)
   	signal.Notify(c, syscall.SIGTERM)
   	select {
   	case <-c:
   		cancel()
   
   	}
}

func setupSlack() {
	// Setup slack adapter.
	slackConfig := slack.NewConfig()
	slackConfig.Token = "REPLACE THIS"
	adapter, err := slack.NewAdapter(slackConfig, slack.WithRTMPayloadHandler(slack.DefaultRTMPayloadHandler))
	if err != nil {
		panic(fmt.Errorf("faileld to setup Slack Adapter: %s", err.Error()))
	}

	// Setup optional storage so conversational context can be stored.
	cacheConfig := sarah.NewCacheConfig()
	storage := sarah.NewUserContextStorage(cacheConfig)

	// Setup Bot with slack adapter and default storage.
	bot := sarah.NewBot(adapter, sarah.BotWithStorage(storage))
	
	sarah.RegisterBot(bot)
}
```

---

```go
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

---

```go
package hello

import (
	"context"
	"github.com/oklahomer/go-sarah/v4"
	"github.com/oklahomer/go-sarah/v4/slack"
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

# Supported Golang Versions
Official [Release Policy](https://golang.org/doc/devel/release.html#policy) says "each major Go release is supported
until there are two newer major releases." Following this policy would help this project enjoy the improvements
introduced in the later versions. However, not all projects can immediately switch to a newer environment. 
Migration could especially be difficult when this project cuts off the older version's support right after a new major Go
release.

As a transition period, this project includes support for one older version than Go project does.
Such a version is guaranteed to be listed in [.travis.ci](https://github.com/oklahomer/go-sarah/blob/master/.travis.yml).
In other words, new features/interfaces introduced in 1.10 can be used in this project only after 1.12 is out.

# Further Readings
- [Project wiki](https://github.com/oklahomer/go-sarah/wiki)
- [GoDoc](https://godoc.org/github.com/oklahomer/go-sarah)
- [Example codes](https://github.com/oklahomer/go-sarah/tree/master/examples)
