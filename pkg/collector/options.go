package collector

import "time"

type Options struct {
	Since     time.Duration
	Namespace string
	Output    string
}

func DefaultOptions() Options {
	return Options{
		Since:     30 * time.Minute,
		Namespace: "",
		Output:    "snapshot.json",
	}
}
