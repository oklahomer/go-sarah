package sarah

import "testing"

var (
	FOO BotType = "foo"
)

func TestBotType(t *testing.T) {
	if FOO.String() != "foo" {
		t.Errorf("BotTYpe does not return expected value. expected 'foo', but was '%s'.", FOO.String())
	}
}

func resetStashedBuilder() {
	stashedCommandBuilder = map[BotType][]*commandBuilder{}
}

type nullCommand struct {
}

func (c *nullCommand) Identifier() string {
	return "fooBarBuzz"
}

func (c *nullCommand) Execute(input BotInput) (*CommandResponse, error) {
	return nil, nil
}

func (c *nullCommand) Example() string {
	return "dummy"
}

func (c *nullCommand) Match(input string) bool {
	return true
}

func (c *nullCommand) StripCommand(input string) string {
	return input
}

func TestAppendCommandBuilder(t *testing.T) {
	resetStashedBuilder()
	commandBuilder :=
		NewCommandBuilder().
			ConfigStruct(NullConfig).
			Identifier("fooCommand").
			Constructor(func(conf CommandConfig) Command { return &nullCommand{} })
	AppendCommandBuilder(FOO, commandBuilder)

	stashedBuilders := stashedCommandBuilder[FOO]
	if size := len(stashedBuilders); size != 1 {
		t.Errorf("1 commandBuilder was expected to be stashed, but was %d", size)
	}
	if builder := stashedBuilders[0]; builder != commandBuilder {
		t.Errorf("stashed commandBuilder is somewhat different. %#v", builder)
	}
}
