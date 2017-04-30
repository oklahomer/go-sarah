package worldweather

import (
	"encoding/json"
	"io/ioutil"
	"path/filepath"
	"testing"
)

func TestHourlyForecastTime_UnmarshalText(t *testing.T) {
	data := []struct {
		givenValue  string
		displayTime string
		hour        int
		hasError    bool
	}{
		{
			givenValue:  "0",
			displayTime: "00:00",
			hour:        0,
		},
		{
			givenValue:  "0300",
			displayTime: "03:00",
			hour:        3,
		},
		{
			givenValue:  "1200",
			displayTime: "12:00",
			hour:        12,
		},
		{
			givenValue: "bad string",
			hasError:   true,
		},
	}

	for _, datum := range data {
		forecastTime := &HourlyForecastTime{}
		err := forecastTime.UnmarshalText([]byte(datum.givenValue))

		if !datum.hasError && err != nil {
			t.Errorf("Unexpected error is returned: %s.", err.Error())
			continue
		} else if datum.hasError && err == nil {
			t.Error("Expected error is not returned.")
			continue
		}

		if forecastTime.DisplayTime != datum.displayTime {
			t.Errorf("Unexpected display time is returned: %s.", forecastTime.DisplayTime)
		}

		if forecastTime.Hour != datum.hour {
			t.Errorf("Unexpected hour value is returned: %d.", forecastTime.Hour)
		}
	}
}

func TestUnmarshalLocalWeatherResponse(t *testing.T) {
	path, err := filepath.Abs(filepath.Join("..", "..", "testdata", "plugins", "worldweather", "weather.json"))
	if err != nil {
		t.Fatalf("Test file could not be located: %s.", err.Error())
	}

	buf, err := ioutil.ReadFile(path)
	if err != nil {
		t.Fatalf("Test data could not be loaded: %s.", err.Error())
	}

	response := &LocalWeatherResponse{}
	err = json.Unmarshal(buf, response)

	if err != nil {
		t.Fatalf("Unexpected error is returned: %s.", err.Error())
	}

	if response.Data.HasError() {
		t.Fatalf("Unexpected error value is returned: %s.", response.Data.Error[0].Message)
	}
}
