package sarah

import (
	"golang.org/x/net/context"
	"testing"
)

func TestStashScheduledTaskBuilder(t *testing.T) {
	stashedScheduledTaskBuilders = &scheduledTaskBuilderStash{}

	var botType BotType = "myBot"
	builder := NewScheduledTaskBuilder()
	StashScheduledTaskBuilder(botType, builder)

	if len((*stashedScheduledTaskBuilders)[botType]) != 1 {
		t.Fatal("Given builder is no stashed")
	}

	if (*stashedScheduledTaskBuilders)[botType][0] != builder {
		t.Fatal("Stashed builder is not the given one.")
	}
}

func TestScheduledTaskBuilderStash_build(t *testing.T) {
	var botType BotType = "myBot"
	stashedScheduledTaskBuilders = &scheduledTaskBuilderStash{}
	emptyBuild := stashedScheduledTaskBuilders.build(botType, "")
	if emptyBuild == nil || len(emptyBuild) != 0 {
		t.Fatalf("Empty slice MUST be returned even when no builder is fed: %#v.", emptyBuild)
	}

	invalidBuilder := NewScheduledTaskBuilder()
	StashScheduledTaskBuilder(botType, invalidBuilder)

	commandID := "scheduled"
	config := &DummyScheduledTaskConfig{}
	validBuilder := NewScheduledTaskBuilder().
		ConfigurableFunc(config, func(_ context.Context, _ TaskConfig) ([]*ScheduledTaskResult, error) {
			return nil, nil
		}).
		Identifier(commandID)
	StashScheduledTaskBuilder(botType, validBuilder)

	commands := stashedScheduledTaskBuilders.build(botType, "testdata/taskbuilder")
	if len(commands) != 1 {
		t.Fatalf("Expecting 1 command to be built, but was: %d.", len(commands))
	}

	if commands[0].Identifier() != commandID {
		t.Fatalf("Unexpected command is returned: %#v.", commands[0])
	}
}
