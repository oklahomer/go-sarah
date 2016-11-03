package sarah

import (
	"github.com/oklahomer/go-sarah/log"
)

var (
	stashedCommandBuilders       = &commandBuilderStash{}
	stashedScheduledTaskBuilders = &scheduledTaskBuilderStash{}
)

type commandBuilderStash map[BotType][]*commandBuilder

// AppendCommandBuilder appends given commandBuilder to internal stash.
// Stashed builder is used to configure and build Command instance on Runner's initialization.
//
// They are built in appended order, which means commands are checked against user input in the appended order.
// Therefore, append commands with higher priority or narrower regular expression match pattern.
func AppendCommandBuilder(botType BotType, builder *commandBuilder) {
	log.Infof("appending command builder for %s. builder %#v.", botType, builder)
	stashedCommandBuilders.appendBuilder(botType, builder)
}

func (stash *commandBuilderStash) appendBuilder(botType BotType, builder *commandBuilder) {
	val := *stash
	if _, ok := val[botType]; !ok {
		val[botType] = make([]*commandBuilder, 0)
	}
	val[botType] = append(val[botType], builder)
}

func (stash *commandBuilderStash) build(botType BotType, configDir string) []Command {
	commands := []Command{}
	builders, ok := (*stash)[botType]
	if !ok {
		return commands
	}

	for _, builder := range builders {
		command, err := builder.build(configDir)
		if err != nil {
			log.Errorf("can't configure plugin: %s. error: %s.", builder.identifier, err.Error())
			continue
		}
		commands = append(commands, command)
	}

	return commands
}

type scheduledTaskBuilderStash map[BotType][]*scheduledTaskBuilder

// AppendScheduledTaskBuilder appends given scheduledTaskBuilder to internal stash.
// Stashed builder is used to configure and build ScheduledTask instance on Runner's initialization.
func AppendScheduledTaskBuilder(botType BotType, builder *scheduledTaskBuilder) {
	log.Infof("appending scheduled task builder for %s. builder %#v.", botType, builder)
	stashedScheduledTaskBuilders.appendBuilder(botType, builder)
}

func (stash *scheduledTaskBuilderStash) appendBuilder(botType BotType, builder *scheduledTaskBuilder) {
	val := *stash
	if _, ok := val[botType]; !ok {
		val[botType] = make([]*scheduledTaskBuilder, 0)
	}
	val[botType] = append(val[botType], builder)
}

// buildCommands configures and creates Command instances with given stashed CommandBuilders
func (stash *scheduledTaskBuilderStash) build(botType BotType, configDir string) []*scheduledTask {
	tasks := []*scheduledTask{}
	builders, ok := (*stash)[botType]
	if !ok {
		return tasks
	}

	for _, builder := range builders {
		task, err := builder.build(configDir)
		if err != nil {
			log.Errorf("can't configure scheduled task plugin: %s. error: %s.", builder.identifier, err.Error())
			continue
		}
		tasks = append(tasks, task)
	}

	return tasks
}
