package sarah

import "testing"

func TestBotType_String(t *testing.T) {
	var BAR BotType = "myNewBotType"
	if BAR.String() != "myNewBotType" {
		t.Errorf("Expected BotType was 'myNewBotType,' but was %s", BAR.String())
	}
}
