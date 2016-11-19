package sarah

import (
	"github.com/oklahomer/go-sarah/log"
)

var (
	stashedCommandBuilders       = &commandBuilderStash{}
	stashedScheduledTaskBuilders = &scheduledTaskBuilderStash{}
)

type commandBuilderStash map[BotType][]*CommandBuilder

// AppendCommandBuilder appends given CommandBuilder to internal stash.
// Stashed builder is used to configure and build Command instance on Runner's initialization.
//
// They are built in appended order, which means commands are checked against user input in the appended order.
// Therefore, append commands with higher priority or narrower regular expression match pattern.
func AppendCommandBuilder(botType BotType, builder *CommandBuilder) {
	log.Infof("appending command builder for %s. builder %#v.", botType, builder)
	stashedCommandBuilders.appendBuilder(botType, builder)
}

func (stash *commandBuilderStash) appendBuilder(botType BotType, builder *CommandBuilder) {
	val := *stash
	if _, ok := val[botType]; !ok {
		val[botType] = make([]*CommandBuilder, 0)
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
		command, err := builder.Build(configDir)
		if err != nil {
			log.Errorf("can't configure command. %s. %#v", err.Error(), builder)
			continue
		}
		commands = append(commands, command)
	}

	return commands
}

type scheduledTaskBuilderStash map[BotType][]*ScheduledTaskBuilder

// AppendScheduledTaskBuilder appends given ScheduledTaskBuilder to internal stash.
// Stashed builder is used to configure and build ScheduledTask instance on Runner's initialization.
func AppendScheduledTaskBuilder(botType BotType, builder *ScheduledTaskBuilder) {
	log.Infof("appending scheduled task builder for %s. builder %#v.", botType, builder)
	stashedScheduledTaskBuilders.appendBuilder(botType, builder)
}

func (stash *scheduledTaskBuilderStash) appendBuilder(botType BotType, builder *ScheduledTaskBuilder) {
	val := *stash
	if _, ok := val[botType]; !ok {
		val[botType] = make([]*ScheduledTaskBuilder, 0)
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
			log.Errorf("can't configure scheduled task: %s. %#v.", err.Error(), builder)
			continue
		}
		tasks = append(tasks, task)
	}

	return tasks
}
