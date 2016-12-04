package sarah

type DummyScheduledTaskConfig struct {
	ScheduleValue    string
	DestinationValue OutputDestination
}

func (config *DummyScheduledTaskConfig) Schedule() string {
	return config.ScheduleValue
}

func (config *DummyScheduledTaskConfig) Destination() OutputDestination {
	return config.DestinationValue
}
