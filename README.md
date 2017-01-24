[![Build Status](https://travis-ci.org/oklahomer/go-sarah.svg?branch=master)](https://travis-ci.org/oklahomer/go-sarah) [![Coverage Status](https://coveralls.io/repos/github/oklahomer/go-sarah/badge.svg?branch=master)](https://coveralls.io/github/oklahomer/go-sarah?branch=master)

Sarah is a general purpose bot framework named after author's firstborn daughter.

While the first goal is to prep author to write Go-ish code, the second goal is to provide simple yet highly customizable bot framework.

# Components

## Runner
Runner is the core of Sarah; It manages other components' lifecycle, handles concurrency with internal workers, watch configuration file changes, **re**-configures commands/tasks on file changes, execute scheduled tasks, and most importantly make Sarah comes alive.

Runner may take multiple Bot implementations to run multiple Bots in single process, so resources such as workers can be shared.

## Bot / Adapter
Bot is responsible for actual interaction with chat services such as Slack, LINE, gitter, etc...

Bot receives messages from chat services, see if the sending user is in the middle of *user context*, search for corresponding command, execute command, and send response back to chat service.

Important thing to be aware of is that, once Bot receives message from chat service, it sends the input to Runner via a channel.
Runner then dispatch a job to internal worker, which calls sarah.Bot.Respond and sends response via Bot.SendMessage.
In other words, after sending input via channel, things are done in concurrent manner without any additional work.

### DefaultBot
Technically Bot is just an interface. So, if desired, developers can create their own Bot implementations to interact with preferred chat services.
However most Bots have similar functionalities, and it is truly cumbersome to implement one for every chat service of choice.

So defaultBot is already predefined. This can be initialized via sarah.NewBot.

### Adapter
sarah.NewBot takes two arguments: Adapter implementation and sarah.CacheConfig.
This Adapter thing becomes a bridge between defaultBot and chat services.
DefaultBot takes care of finding corresponding command against given input, handle cached user context and other miscellaneous tasks; Adapter takes care of connecting/requesting to and sending/receiving from chat service.

```go
package main

import	(
        "github.com/oklahomer/go-sarah"
        "github.com/oklahomer/go-sarah/slack"
        "gopkg.in/yaml.v2"
        "io/ioutil"
)

func main() {
        // Setup slack bot and register desired Command(s).
        // Any Bot implementation can be fed to Runner.RegisterBot(), but for convenience slack and gitter adapters are predefined.
        // sarah.NewBot takes adapter and returns defaultBot instance, which satisfies Bot interface.
        configBuf, _ := ioutil.ReadFile("/path/to/adapter/config.yaml")
        slackConfig := slack.NewConfig() // config struct is returned with default settings.
        yaml.Unmarshal(configBuf, slackConfig)
        _ = sarah.NewBot(slack.NewAdapter(slackConfig), sarah.NewCacheConfig())
}
```

## Command
Command is a plugin that receives user input and return response.
Command.Match is called against user input in Bot.Respond. If it returns *true*, the command is considered *"corresponds to user input,"* and hence its Execute method is called.

Any struct that satisfies Command interface can be fed to Bot.AppendCommand as a command.
CommandBuilder is provided to easily implement Component interface:

### Simple Command

```go
package echo

import (
	"github.com/oklahomer/go-sarah"
	"golang.org/x/net/context"
	"regexp"
)

var matchPattern = regexp.MustCompile(`^\.echo`)

// This can be fed to bot via slackBot.AppendCommand(echo.SlackCommand)
var SlackCommand = sarah.NewCommandBuilder().
        Identifier("echo").
        MatchPattern(matchPattern).
        Func(func(_ context.Context, input sarah.Input) (*sarah.CommandResponse, error) {
                // ".echo foo" to "foo"
                return sarah.NewStringResponse(sarah.StripMessage(matchPattern, input.Message())), nil
        }).
        InputExample(".echo knock knock").
        MustBuild()
```

### Reconfigurable Command
With CommandBuilder.ConfigurableFunc, a desired configuration struct may be added.
This configuration struct is passed on command execution as 3rd argument.
Runner is watching the changes on configuration files' directory and if configuration file is updated, then the corresponding command is built, again.

## Scheduled Task
While commands are set of functions that responds to user input, scheduled task is one that runs in scheduled manner.
e.g. Say "Good morning, sir!" every 7:00 a.m., search on database and sends "today's chores list" to each specific room, etc...

### Simple Scheduled Task

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
With ScheduledTaskBuilder.ConfigurableFunc, a desired configuration struct may be added.
This configuration struct is passed on task execution as 2nd argument.
Runner is watching the changes on configuration files' directory and if configuration file is updated, then the corresponding task is built/scheduled, again.

# Features

## User context
To be declared...

## Live Configuration Update
To be declared...

# Getting Started

It is pretty easy to add support for developers' choice of chat service, but this supports Slack and Gitter out of the box as reference implementations.

Configuration for Slack goes like below:

```Go
package main

import	(
        "github.com/oklahomer/go-sarah"
        "github.com/oklahomer/go-sarah/plugins/hello"
        "github.com/oklahomer/go-sarah/slack"
        "github.com/oklahomer/golack/rtmapi"
        "golang.org/x/net/context"
        "gopkg.in/yaml.v2"
        "io/ioutil"
        "regexp"
        "time"
)

func main() {
        // Setup slack bot and register desired Command(s).
        // Any Bot implementation can be fed to Runner.RegisterBot(), but for convenience slack and gitter adapters are predefined.
        // sarah.NewBot takes adapter and returns defaultBot instance, which satisfies Bot interface.
        configBuf, _ := ioutil.ReadFile("/path/to/adapter/config.yaml")
        slackConfig := slack.NewConfig()
        yaml.Unmarshal(configBuf, slackConfig)
        slackBot := sarah.NewBot(slack.NewAdapter(slackConfig), sarah.NewCacheConfig())

        // Register desired command
        slackBot.AppendCommand(hello.Command)

        // Create a builder for simple command that requires no config struct.
        // sarah.StashCommandBuilder can be used to stash this builder and build Command on Runner.Run,
        // or use Build() / MustBuild() to build by hand.
        //
        // MustBuild() simplifies safe initialization of global variables holding built Command instance.
        // e.g. Define echo package and expose echo.Command for later use with bot.AppendCommand(echo.Command).
        echoCommand := sarah.NewCommandBuilder().
                Identifier("echo").
                MatchPattern(regexp.MustCompile(`^\.echo`)).
                Func(func(_ context.Context, input sarah.Input) (*sarah.CommandResponse, error) {
                        return sarah.NewStringResponse(input.Message()), nil
                }).
                InputExample(".echo knock knock").
                MustBuild()
        slackBot.AppendCommand(echoCommand)

        // Create a builder for a bit complex command that requires config struct.
        // Configuration file is lazily read on Runner.Run, and command is built with fully configured config struct.
        // The path to the configuration file MUST be equivalent to below:
        //
        //   filepath.Join(sarah.Config.PluginConfigRoot, Bot.BotType(), Command.Identifier() + ".yaml")
        //
        // When configuration file is updated, runner will notify and rebuild the command to apply.
        pluginConfig := &struct{
                Token string `yaml:"api_key"`
        }{}
        configCommandBuilder := sarah.NewCommandBuilder().
                Identifier("configurableCommandSample").
                MatchPattern(regexp.MustCompile(`^\.complexCommand`)).
                ConfigurableFunc(pluginConfig, func(_ context.Context, input sarah.Input, config sarah.Config) (*sarah.CommandResponse, error) {
                        return sarah.NewStringResponse("return something"), nil
                }).
                InputExample(".echo knock knock")
        sarah.StashCommandBuilder(slack.SLACK, configCommandBuilder)

        // Initialize Runner
        config := sarah.NewConfig()
        config.PluginConfigRoot = "path/to/plugin/configuration" // can be set manually or with (json|yaml).Unmarshal
        runner := sarah.NewRunner(config)

        // Register declared bot.
        runner.RegisterBot(slackBot)

        // Start interaction
        rootCtx := context.Background()
        runnerCtx, cancelRunner := context.WithCancel(rootCtx)
        runner.Run(runnerCtx)

        // Register scheduled task that require no configuration.
        sarah.NewScheduledTaskBuilder().Identifier("scheduled").Func
        task := sarah.NewScheduledTaskBuilder().
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
		runner.RegisterScheduledTask(slack.SLACK, task)

        // Let runner run for 30 seconds and eventually stop it by context cancelation.
        time.Sleep(30 * time.Second)
        cancelRunner()
}
```