package worldweather

import (
	"fmt"
	"strconv"
)

// ErrorDescription represents error that returned by World Weather API.
// `{ "data": { "error": [ {"msg": "Unable to find any matching weather location to the query submitted!" } ] }}`
type ErrorDescription struct {
	Message string `json:"msg"`
}

// CommonData represents common response fields returned as part of API response.
type CommonData struct {
	Error []*ErrorDescription `json:"error"`
}

// HasError tells if the response contains any error.
func (data *CommonData) HasError() bool {
	return len(data.Error) > 0
}

// LocalWeatherResponse represents local weather information.
// https://developer.worldweatheronline.com/api/docs/local-city-town-weather-api.aspx#data_element
type LocalWeatherResponse struct {
	Data *WeatherData `json:"data"`
}

// WeatherData represents set of weather information.
type WeatherData struct {
	CommonData
	Request          []*Request          `json:"request"`
	CurrentCondition []*CurrentCondition `json:"current_condition"`
	Weather          []*Weather          `json:"weather"`
}

// Request represents clients request.
type Request struct {
	Type  string `json:"type"`
	Query string `json:"query"`
}

// CurrentCondition represents current weather condition returned by API.
type CurrentCondition struct {
	ObservationTime       string                `json:"observation_time"`
	Temperature           int                   `json:"temp_C,string"`
	FeelingTemperature    int                   `json:"FeelsLikeC,string"`
	WindSpeed             int                   `json:"windspeedKmph,string"`
	WindDirection         int                   `json:"winddirDegree,string"`
	WindDirectionCardinal string                `json:"winddir16Point"`
	WeatherCode           string                `json:"weatherCode"`
	WeatherIcon           []*WeatherIcon        `json:"weatherIconUrl"`
	Description           []*WeatherDescription `json:"weatherDesc"`
	Precipitation         float32               `json:"precpMM,string"`     // Precipitation in mm
	Humidity              int                   `json:"humidity,string"`    // Humidity in percentage
	Visibility            int                   `json:"visibility,string"`  // Visibility in kilometres
	Pressure              int                   `json:"pressure,string"`    // Atmospheric pressure in millibars
	CloudCover            int                   `json:"cloudcocver,string"` // Cloud cover amount in percentage (%)
}

// WeatherIcon is an icon url that represents corresponding weather.
type WeatherIcon struct {
	URL string `json:"value"`
}

// WeatherDescription represents weather description.
type WeatherDescription struct {
	Content string `json:"value"`
}

// Weather represents set of weather information.
type Weather struct {
	Astronomy []*Astronomy     `json:"astronomy"`
	Date      string           `json:"date"` // 2016-09-04
	MaxTempC  int              `json:"maxTempC,string"`
	MaxTempF  int              `json:"maxTempF,string"`
	MinTempC  int              `json:"minTempC,string"`
	MinTempF  int              `json:"minTempF,string"`
	UV        int              `json:"uvindex,string"`
	Hourly    []*HourlyWeather `json:"hourly"`
}

// HourlyWeather represents hourly weather information.
type HourlyWeather struct {
	Time                  HourlyForecastTime    `json:"time"`         // hhmm format
	Temperature           int                   `json:"tempC,string"` // not temp_C
	FeelingTemperature    int                   `json:"FeelsLikeC,string"`
	WindSpeed             int                   `json:"windspeedKmph,string"`
	WindDirection         int                   `json:"winddirDegree,string"`
	WindDirectionCardinal string                `json:"winddir16Point"`
	WeatherCode           string                `json:"weatherCode"`
	WeatherIcon           []*WeatherIcon        `json:"weatherIconUrl"`
	Description           []*WeatherDescription `json:"weatherDesc"`
	Precipitation         float32               `json:"precpMM,string"`     // Precipitation in mm
	Humidity              int                   `json:"humidity,string"`    // Humidity in percentage
	Visibility            int                   `json:"visibility,string"`  // Visibility in kilometres
	Pressure              int                   `json:"pressure,string"`    // Atmospheric pressure in millibars
	CloudCover            int                   `json:"cloudcocver,string"` // Cloud cover amount in percentage (%)
}

// HourlyForecastTime is a time representation for hourly forecast.
type HourlyForecastTime struct {
	OriginalValue string
	DisplayTime   string
	Hour          int
}

// UnmarshalText converts time value returned by API to convenient form.
func (t *HourlyForecastTime) UnmarshalText(b []byte) error {
	str := string(b)
	t.OriginalValue = str

	hhmm, err := strconv.Atoi(str)
	if err != nil {
		return err
	}
	hhmmStr := fmt.Sprintf("%04d", hhmm)
	t.DisplayTime = hhmmStr[:2] + ":" + hhmmStr[2:]
	t.Hour, _ = strconv.Atoi(hhmmStr[:2])

	return nil
}

// Astronomy represents astronomical information.
type Astronomy struct {
	Sunrise  string `json:"sunrise"`
	Sunset   string `json:"sunset"`
	MoonRise string `json:"moonrise"`
	MoonSet  string `json:"moonset"`
}
