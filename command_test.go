package sarah

import "testing"

func TestInsufficientSettings(t *testing.T) {
	builder := NewCommandBuilder()

	if _, err := builder.build("/path/"); err == nil {
		t.Error("expected error not given.")
	} else {
		switch err.(type) {
		case *CommandInsufficientArgumentError:
		// O.K.
		default:
			t.Errorf("expected error not given. %#v", err)
		}
	}

	builder.Identifier("someID")
	if _, err := builder.build("/path/"); err == nil {
		t.Error("expected error not given.")
	} else {
		switch err.(type) {
		case *CommandInsufficientArgumentError:
		// O.K.
		default:
			t.Errorf("expected error not given. %#v", err)
		}
	}

	builder.Constructor(
		func(conf CommandConfig) Command {
			return nil
		},
	)
	if _, err := builder.build("/path/"); err == nil {
		t.Error("expected error not given.")
	} else {
		switch err.(type) {
		case *CommandInsufficientArgumentError:
		// O.K.
		default:
			t.Errorf("expected error not given. %#v", err)
		}
	}

	builder.ConfigStruct(&EmptyCommandConfig{})
	if _, err := builder.build("/path/"); err != nil {
		t.Errorf("something is wrong with command construction. %#v", err)
	}
}
