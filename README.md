[![Go Report Card](https://goreportcard.com/badge/github.com/oklahomer/go-sarah)](https://goreportcard.com/report/github.com/oklahomer/go-sarah) [![Build Status](https://travis-ci.org/oklahomer/go-sarah.svg?branch=master)](https://travis-ci.org/oklahomer/go-sarah) [![Coverage Status](https://coveralls.io/repos/github/oklahomer/go-sarah/badge.svg?branch=master)](https://coveralls.io/github/oklahomer/go-sarah?branch=master)

Sarah is a general purpose bot framework named after author's firstborn daughter.

While the first goal is to prep author to write Go-ish code, the second goal is to provide simple yet highly customizable bot framework.

# Version
Sarah is currently at her Pre-Alpha stage, and is under active development. Interfaces are subjects to change without notice.

Author plans to release 1.0 by actual Sarah's birthday, but does not disclose what year it will be.

# Components

## Runner
```Runner``` is the core of Sarah; It manages other components' lifecycle, handles concurrency with internal workers, watch configuration file changes, **re**-configures commands/tasks on file changes, execute scheduled tasks, and most importantly make Sarah comes alive.

```Runner``` may take multiple ```Bot``` implementations to run multiple Bots in single process, so resources such as workers can be shared.

## Bot / Adapter
```Bot``` interface is responsible for actual interaction with chat services such as Slack, [LINE](https://github.com/oklahomer/go-sarah-line), gitter, etc...

```Bot``` receives messages from chat services, see if the sending user is in the middle of *user context*, search for corresponding ```Command```, execute ```Command```, and send response back to chat service.

Important thing to be aware of is that, once ```Bot``` receives message from chat service, it sends the input to ```Runner``` via a channel.
```Runner``` then dispatch a job to internal worker, which calls ```Bot.Respond``` and sends response via ```Bot.SendMessage```.
In other words, after sending input via channel, things are done in concurrent manner without any additional work.
Change worker configuration to throttle the number of concurrent execution -- this may also impact the number of concurrent HTTP requests against chat service provider.

### DefaultBot
Technically ```Bot``` is just an interface. So, if desired, developers can create their own ```Bot``` implementations to interact with preferred chat services.
However most Bots have similar functionalities, and it is truly cumbersome to implement one for every chat service of choice.

So ```defaultBot``` is already predefined. This can be initialized via ```sarah.NewBot```.

### Adapter
```sarah.NewBot``` takes two arguments: ```Adapter``` implementation and ```sarah.CacheConfig```.
This ```Adapter``` thing becomes a bridge between defaultBot and chat service.
```DefaultBot``` takes care of finding corresponding command against given input, handling cached user context, and other miscellaneous tasks; ```Adapter``` takes care of connecting/requesting to and sending/receiving from chat service.

```go
package main

import	(
        "github.com/oklahomer/go-sarah"
        "github.com/oklahomer/go-sarah/slack"
        "gopkg.in/yaml.v2"
        "io/ioutil"
)

func main() {
        // Setup slack bot.
        // Any Bot implementation can be fed to Runner.RegisterBot(), but for convenience slack and gitter adapters are predefined.
        // sarah.NewBot takes adapter and returns defaultBot instance, which satisfies Bot interface.
        configBuf, _ := ioutil.ReadFile("/path/to/adapter/config.yaml")
        slackConfig := slack.NewConfig() // config struct is returned with default settings.
        yaml.Unmarshal(configBuf, slackConfig)
        _ = sarah.NewBot(slack.NewAdapter(slackConfig), sarah.NewCacheConfig())
}
```

## Command
```Command``` interface represents a plugin that receives user input and return response.
```Command.Match``` is called against user input in ```Bot.Respond```. If it returns *true*, the command is considered *"corresponds to user input,"* and hence its ```Execute``` method is called.

Any struct that satisfies ```Command``` interface can be fed to ```Bot.AppendCommand``` as a command.
```CommandBuilder``` is provided to easily implement ```Command``` interface on the fly:

### Simple Command

```go
package echo

import (
	"github.com/oklahomer/go-sarah"
	"github.com/oklahomer/go-sarah/slack"
	"golang.org/x/net/context"
	"regexp"
)

var matchPattern = regexp.MustCompile(`^\.echo`)

// This can be fed to bot via Bot.AppendCommand
var SlackCommand = sarah.NewCommandBuilder().
        Identifier("echo").
        MatchPattern(matchPattern).
        Func(func(_ context.Context, input sarah.Input) (*sarah.CommandResponse, error) {
                // ".echo foo" to "foo"
                return slack.NewStringResponse(sarah.StripMessage(matchPattern, input.Message())), nil
        }).
        InputExample(".echo knock knock").
        MustBuild()
```

### Reconfigurable Command
With ```CommandBuilder.ConfigurableFunc```, a desired configuration struct may be added.
This configuration struct is passed on command execution as 3rd argument.
```Runner``` is watching the changes on configuration files' directory and if configuration file is updated, then the corresponding command is built, again.

## Scheduled Task
While commands are set of functions that responds to user input, scheduled task is one that runs in scheduled manner.
e.g. Say "Good morning, sir!" every 7:00 a.m., search on database and send "today's chores list" to each specific room, etc...

```ScheduledTask``` implementation can be fed to ```Runner.RegisterScheduledTask```.
When ```Runner.Run``` is called, clock starts to tick and scheduled task become active; Tasks will be executed as scheduled, and results are sent to chat service via ```Bot.SendMessage```.

### Simple Scheduled Task
Technically any struct that satisfies ```ScheduledTask``` interface can be treated as scheduled task, but a builder is provided to construct a ```ScheduledTask``` on the fly.

```go
package foo

import (
	"github.com/oklahomer/go-sarah"
	"github.com/oklahomer/golack/rtmapi"
	"golang.org/x/net/context"
)

var Task = sarah.NewScheduledTaskBuilder().
        Identifier("greeting").
        Func(func(_ context.Context) ([]*sarah.ScheduledTaskResult, error) {
                return []*sarah.ScheduledTaskResult{
				        {
		                        Content:     "Howdy!!",
					            Destination: &rtmapi.Channel{Name: "XXXX"},
				        },
			    }, nil
		}).
        Schedule("@everyday").
		MustBuild()
```

### Reconfigurable Scheduled Task
With ```ScheduledTaskBuilder.ConfigurableFunc```, a desired configuration struct may be added.
This configuration struct is passed on task execution as 2nd argument.
```Runner``` is watching the changes on configuration files' directory and if configuration file is updated, then the corresponding task is built/scheduled, again.

# Features

## User context
To be declared...

## Live Configuration Update
To be declared...

# Getting Started

It is pretty easy to add support for developers' choice of chat service, but this supports Slack, [LINE](https://github.com/oklahomer/go-sarah-line), and Gitter out of the box as reference implementations.

Configuration for Slack goes like below:

```Go
package main

import	(
        "github.com/oklahomer/go-sarah"
        "github.com/oklahomer/go-sarah/log"
        "github.com/oklahomer/go-sarah/plugins/hello"
        "github.com/oklahomer/go-sarah/slack"
        "golang.org/x/net/context"
        "gopkg.in/yaml.v2"
        "io/ioutil"
        "os"
        "os/signal"
        "syscall"
)

func main() {
        // Setup slack bot and register desired Command(s).
        // Any Bot implementation can be fed to Runner.RegisterBot(), but for convenience slack and gitter adapters are predefined.
        // sarah.NewBot takes adapter and returns defaultBot instance, which satisfies Bot interface.
        configBuf, _ := ioutil.ReadFile("/path/to/adapter/config.yaml")
        slackConfig := slack.NewConfig()
        yaml.Unmarshal(configBuf, slackConfig)
        slackBot := sarah.NewBot(slack.NewAdapter(slackConfig), sarah.NewCacheConfig())

        // Register desired command(s)
        slackBot.AppendCommand(hello.SlackCommand)

        // Initialize Runner
        config := sarah.NewConfig()
        config.PluginConfigRoot = "path/to/plugin/configuration" // can be set manually or with (json|yaml).Unmarshal
        runner := sarah.NewRunner(config)

        // Register declared bot.
        runner.RegisterBot(slackBot)

        // Start interaction
        rootCtx := context.Background()
        runnerCtx, cancelRunner := context.WithCancel(rootCtx)
        runnerStop := make(chan struct{})
        go func() {
                runner.Run(runnerCtx) // Blocks til all registered Bots stop
                runnerStop <- struct{}{}
        }()

        // Receives signal to stop Runner.
        c := make(chan os.Signal, 1)
        signal.Notify(c, os.Interrupt)
        signal.Notify(c, syscall.SIGTERM)
        select {
        case <-c:
		        log.Info("Canceled Runner.")
		        cancelRunner()
        case <-runnerStop:
                log.Error("Runner stopped.")
                // Stop because all bots stopped.
	    }
}
```