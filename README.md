[![GoDoc](https://godoc.org/github.com/oklahomer/go-sarah?status.svg)](https://godoc.org/github.com/oklahomer/go-sarah)
[![Go Report Card](https://goreportcard.com/badge/github.com/oklahomer/go-sarah)](https://goreportcard.com/report/github.com/oklahomer/go-sarah)
[![Build Status](https://travis-ci.org/oklahomer/go-sarah.svg?branch=master)](https://travis-ci.org/oklahomer/go-sarah)
[![Coverage Status](https://coveralls.io/repos/github/oklahomer/go-sarah/badge.svg?branch=master)](https://coveralls.io/github/oklahomer/go-sarah?branch=master)

Sarah is a general purpose bot framework named after author's firstborn daughter.

While the first goal is to prep author to write Go-ish code, the second goal is to provide simple yet highly customizable bot framework.

# Example Codes
For those who like to go ahead and see runnable code, some example codes are located under [./examples](https://github.com/oklahomer/go-sarah/tree/master/examples).

# Notable Features

## User's Conversational Context
In this project, user's conversational context is referred to as "**user context**," which stores previous user states and defines what function should be executed on next user input.
While typical bot implementation is somewhat "**stateless**" and hence user-bot interaction does not consider previous state, Sarah natively supports the idea of this conversational context.
Its aim is to let user provide information as they send messages, and finally build up complex command arguments.

For example, instead of obligating user to input long confusing text such as ".todo Fix Sarah's issue #123 by 2017-04-15 12:00:00" at once, let user build up arguments in a conversational manner as below:
![conversational context example](/doc/img/conoversational_context.png)

## Live Configuration Update
When configuration file for a command is updated,
Sarah automatically detect the event and re-build the command or scheduled task in thread-safe manner
so the next execution of that command/task appropriately reflect the new configuration values.

See the usage of ```CommandPropsBuilder``` and ```ScheduledTaskPropsBuilder``` for detail.

## Concurrent Execution by Default
Developers may implement their own bot by a) implementing ```sarah.Bot``` interface or
b) implementing ```sarah.Adapter``` and passing it to ```sarah.NewBot()``` to get instance of default ```Bot``` implementation.

Either way, a component called ```sarah.Runner``` takes care of ```Commmand``` execution against given user input.
This ```sarah.Runner``` dispatches tasks to its internal workers, which means developers do not have to make extra effort to handle flooding incoming messages.

## Alerting Mechanism
When a bot confronts critical situation and can not continue its operation or recover, Sarah's alerting mechanism sends alert to administrator.
Zero or more ```sarah.Alerter``` implementations can be registered to send alert to desired destinations.

## Higher Customizability
To have higher customizability, Sarah is composed of fine grained components that each has one domain to serve:
sarah.Alerter is responsible for sending bot's critical state to administrator,
workers.Worker is responsible for executing given job in a panic-proof manner, etc...
Each component comes with an interface and default implementation,
so developers may change Sarah's behavior by implementing corresponding component's interface and replacing default implementation.

# Components

![component diagram](/doc/uml/component.png)

## Runner
```Runner``` is the core of Sarah; It manages other components' lifecycle, handles concurrent job execution with internal workers, watches configuration file changes, **re**-configures commands/tasks on file changes, executes scheduled tasks, and most importantly makes Sarah comes alive.

```Runner``` may take multiple ```Bot``` implementations to run multiple Bots in single process, so resources such as workers and memory space can be shared.

## Bot / Adapter
```Bot``` interface is responsible for actual interaction with chat services such as Slack, [LINE](https://github.com/oklahomer/go-sarah-line), gitter, etc...
Or if two or more parties are messaging each other over pre-defined protocol and executing corresponding ```Command```, such system can be created by providing one ```Bot``` for each party just like [go-sarah-iot](https://github.com/oklahomer/go-sarah-iot) does to support communication between IoT devices and a central server.

```Bot``` receives messages from chat services, sees if the sending user is in the middle of *user context*, searches for corresponding ```Command```, executes ```Command```, and sends response back to chat service.

Important thing to be aware of is that, once ```Bot``` receives message from chat service, it sends the input to ```Runner``` via a designated channel.
```Runner``` then dispatches a job to internal worker, which calls ```Bot.Respond``` and sends response via ```Bot.SendMessage```.
In other words, after sending input via the channel, things are done in concurrent manner without any additional work.
Change worker configuration to throttle the number of concurrent execution -- this may also impact the number of concurrent HTTP requests against chat service provider.

### DefaultBot
Technically ```Bot``` is just an interface. So, if desired, developers can create their own ```Bot``` implementations to interact with preferred chat services.
However most Bots have similar functionalities, and it is truly cumbersome to implement one for every chat service of choice.

Therefore ```defaultBot``` is already predefined. This can be initialized via ```sarah.NewBot```.

### Adapter
```sarah.NewBot``` takes multiple arguments: ```Adapter``` implementation and arbitrary number of```sarah.DefaultBotOption```s as functional options.
This ```Adapter``` thing becomes a bridge between defaultBot and chat service.
```DefaultBot``` takes care of finding corresponding command against given input, handling stored user context, and other miscellaneous tasks; ```Adapter``` takes care of connecting/requesting to and sending/receiving from chat service.

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
        slackAdapter, _ := slack.NewAdapter(slackConfig)
        sarah.NewBot(slackAdapter)
}
```

## Command
```Command``` interface represents a plugin that receives user input and return response.
```Command.Match``` is called against user input in ```Bot.Respond```. If it returns *true*, the command is considered *"corresponds to user input,"* and hence its ```Execute``` method is called.

Any struct that satisfies ```Command``` interface can be fed to ```Bot.AppendCommand``` as a command.
```CommandPropsBuilder``` is provided to easily implement ```Command``` interface on the fly:

### Simple Command
There are several ways to setup ```Command```s.
- Define a struct that implements ```Command``` interface. Pass its instance to ```Bot.ApendCommand```.
- Use ```CommandPropsBuilder``` to construct a non-contradicting set of arguments, and pass this to ```Runner```.<br />
```Runner``` internally builds a command, and re-built it when configuration struct is present and corresponding configuration file is updated.

Below are some different ways to setup ```CommandProps``` with ```CommandPropsBuilder``` for different customization.

```go
// In separate plugin file such as echo/command.go
// Export some pre-build command props
package echo

import (
	"github.com/oklahomer/go-sarah"
	"github.com/oklahomer/go-sarah/slack"
	"golang.org/x/net/context"
	"regexp"
)

// CommandProps is a set of configuration options that can be and should be treated as one in logical perspective.
// This can be fed to Runner to build Command on the fly.
// CommandProps is re-used when command is re-built due to configuration file update.
var matchPattern = regexp.MustCompile(`^\.echo`)
var SlackProps = sarah.NewCommandPropsBuilder().
        BotType(slack.SLACK).
        Identifier("echo").
        MatchPattern(matchPattern).
        Func(func(_ context.Context, input sarah.Input) (*sarah.CommandResponse, error) {
                // ".echo foo" to "foo"
                return slack.NewStringResponse(sarah.StripMessage(matchPattern, input.Message())), nil
        }).
        InputExample(".echo knock knock").
        MustBuild()

// To have complex checking logic, MatchFunc can be used instead of MatchPattern.
var CustomizedProps = sarah.NewCommandPropsBuilder().
        MatchFunc(func(input sarah.Input) bool {
                // Check against input.Message(), input.SenderKey(), and input.SentAt()
                // to see if particular user is sending particular message in particular time range
                return false
        }).
        // Call some other setter methods to do the rest.
        MustBuild()

// Configurable is a helper function that returns CommandProps built with given CommandConfig.
// CommandConfig can be first configured manually or from YAML/JSON file, and then fed to this function.
// Returned CommandProps can be fed to Runner and when configuration file is updated,
// Runner detects the change and re-build the Command with updated configuration struct.
func Configurable(config sarah.CommandConfig) *sarah.CommandProps {
        return sarah.NewCommandPropsBuilder().
                ConfigurableFunc(config, func(_ context.Context, input sarah.Input, conf sarah.CommandConfig) (*sarah.CommandResponse, error) {
                        return nil, nil
                }).
                // Call some other setter methods to do the rest.
                MustBuild()
}
```

### Reconfigurable Command
With ```CommandPropsBuilder.ConfigurableFunc```, a desired configuration struct may be added.
This configuration struct is passed on command execution as 3rd argument.
```Runner``` is watching the changes on configuration files' directory and if configuration file is updated, then the corresponding command is built, again.

To let Runner supervise file change event, set sarah.Config.PluginConfigRoot.
Internal directory watcher supervises ```sarah.Config.PluginConfigRoot + "/" + BotType + "/"``` as ```Bot```'s configuration directory.
When any file under that directory is updated, ```Runner``` searches for corresponding ```CommandProps``` based on the assumption that the file name is equivalent to ```CommandProps.identifier + ".(yaml|yml|json)""```.
If a corresponding ```CommandProps``` exists, ```Runner``` rebuild ```Command``` with latest configuration values and replaces with the old one.

## Scheduled Task
While commands are set of functions that respond to user input, scheduled tasks are those that run in scheduled manner.
e.g. Say "Good morning, sir!" every 7:00 a.m., search on database and send "today's chores list" to each specific room, etc...

```ScheduledTask``` implementation can be fed to ```Runner.RegisterScheduledTask```.
When ```Runner.Run``` is called, clock starts to tick and scheduled task becomes active; Tasks will be executed as scheduled, and results are sent to chat service via ```Bot.SendMessage```.

### Simple Scheduled Task
Technically any struct that satisfies ```ScheduledTask``` interface can be treated as scheduled task, but a builder is provided to construct a ```ScheduledTask``` on the fly.

```go
package foo

import (
	"github.com/oklahomer/go-sarah"
	"github.com/oklahomer/go-sarah/slack"
	"github.com/oklahomer/golack/slackobject"
	"golang.org/x/net/context"
)

// TaskProps is a set of configuration options that can be and should be treated as one in logical perspective.
// This can be fed to Runner to build ScheduledTask on the fly.
// ScheduledTaskProps is re-used when command is re-built due to configuration file update.
var TaskProps = sarah.NewScheduledTaskPropsBuilder().
        BotType(slack.SLACK).
        Identifier("greeting").
        Func(func(_ context.Context) ([]*sarah.ScheduledTaskResult, error) {
                return []*sarah.ScheduledTaskResult{
                        {
                                Content:     "Howdy!!",
                                Destination: slackobject.ChannelID("XXX"),
                        },
                }, nil
        }).
        Schedule("@everyday").
        MustBuild()
```

### Reconfigurable Scheduled Task
With ```ScheduledTaskPropsBuilder.ConfigurableFunc```, a desired configuration struct may be added.
This configuration struct is passed on task execution as 2nd argument.
```Runner``` is watching the changes on configuration files' directory and if configuration file is updated, then the corresponding task is built/scheduled, again.

To let Runner supervise file change event, set sarah.Config.PluginConfigRoot.
Internal directory watcher supervises ```sarah.Config.PluginConfigRoot + "/" + BotType + "/"``` as ```Bot```'s configuration directory.
When any file under that directory is updated, ```Runner``` searches for corresponding ```ScheduledTaskProps``` based on the assumption that the file name is equivalent to ```ScheduledTaskProps.identifier + ".(yaml|yml|json)""```.
If a corresponding ```ScheduledTaskProps``` exists, ```Runner``` rebuild ```ScheduledTask``` with latest configuration values and replaces with the old one.

## UserContextStorage
As described in "Notable Features," Sarah stores user's current state when ```Command```'s response expects user to send series of messages with extra supplemental information.
```UserContextStorage``` is where the state is stored.
Developers may store state into desired storage by implementing ```UserContextStorage``` interface.
Two implementations are currently provided by author:

### Store in Process Memory Space
```defaultUserContextStorage``` is a ```UserContextStorage``` implementation that stores ```ContextualFunc```, a function to be executed on next user input, in the exact same memory space that process is currently running.
Under the hood this storage is simply a map where key is user identifier and value is ```ContextualFunc```.
This ```ContextFunc``` can be any function including instance method and anonymous function that satisfies ```ContextFunc``` type.
However it is recommended to use anonymous function since some variable declared on last method call can be casually referenced in this scope.

### Store in External KVS
[go-sarah-rediscontext](https://github.com/oklahomer/go-sarah-rediscontext) stores combination of function identifier and serializable arguments in Redis.
This is extremely effective when multiple Bot processes run and user context must be shared among them.

e.g. Chat platform such as LINE sends HTTP requests to Bot on every user input, where Bot may consist of multiple servers/processes to balance those requests.

## Alerter
When registered Bot encounters critical situation and requires administrator's direct attention, ```Runner``` sends alert message as configured with ```Alerter```.
LINE alerter is provided by default, but anything that satisfies ```Alerter``` interface can be registered as ```Alerter```.
Developer may add multiple ```Alerter``` implementations via ```Runner.RegisterAlerter``` so it is recommended to register multiple ```Alerter```s to avoid Alerting channel's malfunction and make sure administrator notices critical state.

Bot/Adapter may send ```BotNonContinurableError``` via error channel to notify critical state to ```Runner```.
e.g. ```Adapter``` can not connect to chat service provider after reasonable number of retrials.

# Getting Started

It is pretty easy to add support for developers' choice of chat service, but this supports Slack, [LINE](https://github.com/oklahomer/go-sarah-line), and Gitter out of the box as reference implementations.

Example setup for Slack is located at [example/main.go](https://github.com/oklahomer/go-sarah/blob/master/examples/main.go).
