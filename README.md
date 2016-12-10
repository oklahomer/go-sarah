[![Build Status](https://travis-ci.org/oklahomer/go-sarah.svg?branch=master)](https://travis-ci.org/oklahomer/go-sarah) [![Coverage Status](https://coveralls.io/repos/github/oklahomer/go-sarah/badge.svg?branch=master)](https://coveralls.io/github/oklahomer/go-sarah?branch=master)

Sarah is a general purpose bot framework named after author's firstborn daughter.

While the first goal is to prep author to write Go-ish code, the second goal is to provide simple yet highly customizable bot framework.
It is pretty easy to add support for developers' choice of chat service, but this supports Slack and Gitter out of the box as reference implementations.

Configuration for Slack goes like below:

```Go
package main

import	(
        "github.com/oklahomer/go-sarah"
        "github.com/oklahomer/go-sarah/slack"
        "golang.org/x/net/context"
        "regexp"
        "time"
)

func main() {
        // Create a builder for simple command that requires no config struct.
        echoBuilder := sarah.NewCommandBuilder().
                Identifier("echo").
                MatchPattern(regexp.MustCompile(`^\.echo`)).
                Func(func(_ context.Context, input sarah.Input) (*sarah.CommandResponse, error) {
                        return slack.NewStringResponse(input.Message()), nil
                }).
                InputExample(".echo knock knock")
        sarah.StashCommandBuilder(slack.SLACK, echoBuilder)

        // Create a builder for a bit complex command that requires config struct.
        // Configuration file is read on Runner.Run, and command is built with fully configured config struct.
        pluginConfig := &struct{
                Token string `yaml:"api_key"`
        }{}
        configCommandBuilder := sarah.NewCommandBuilder().
                Identifier("configurableCommandSample").
                MatchPattern(regexp.MustCompile(`^\.complexCommand`)).
                ConfigurableFunc(pluginConfig, func(_ context.Context, input sarah.Input, config sarah.Config) (*sarah.CommandResponse, error) {
                        return slack.NewStringResponse("return something"), nil
                }).
                InputExample(".echo knock knock")
        sarah.StashCommandBuilder(slack.SLACK, configCommandBuilder)
        
        // Initialize Runner
        runner := sarah.NewRunner(sarah.NewConfig())
        // Setup slack adapter and add this instance to runner
        adapter := slack.NewAdapter(slack.NewConfig("dummyToken"))
        runner.RegisterAdapter(adapter, "/path/to/plugin/config/dir/")

        // Start interaction
        rootCtx := context.Background()
        runnerCtx, cancelRunner := context.WithCancel(rootCtx)
        runner.Run(runnerCtx)

        // Let runner run for 30 seconds and eventually stop it by context cancelation.
        time.Sleep(30 * time.Second)
        cancelRunner()
}
```