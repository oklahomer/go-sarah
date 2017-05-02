package hello

import (
	"testing"
)

func Test_slackFunc(t *testing.T) {
	response, err := slackFunc(nil, nil)
	if err != nil {
		t.Errorf("Unexpected error is returned: %s.", err.Error())
	}

	if response == nil {
		t.Fatal("Exppected response is not returned.")
	}

	if response.UserContext != nil {
		t.Errorf("Unexpected UserContext is returned: %#v.", response.UserContext)
	}

	if str, ok := response.Content.(string); ok {
		if str != "Hello!" {
			t.Errorf("Unexpected text is returned: %s.", str)
		}
	} else {
		t.Errorf("Returned content has unexpected type: %#v", response.Content)
	}
}
