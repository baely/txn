package server

import (
	"time"
)

var snapTimes = []time.Duration{
	time.Duration(1) * time.Second,
	time.Duration(5) * time.Second,
	time.Duration(10) * time.Second,
	time.Duration(30) * time.Second,
	time.Duration(1) * time.Minute,
	time.Duration(5) * time.Minute,
	time.Duration(10) * time.Minute,
	time.Duration(30) * time.Minute,
	time.Duration(1) * time.Hour,
	time.Duration(2) * time.Hour,
	time.Duration(6) * time.Hour,
	time.Duration(12) * time.Hour,
	time.Duration(1) * time.Hour * 24,
}

// snapDownDuration returns the closest snap time that is less than the given duration.
func snapDownDuration(d time.Duration) time.Duration {
	for i, snap := range snapTimes {
		if snap < d {
			continue
		}

		if i == 0 {
			return snap
		}

		return snapTimes[i-1]
	}
	return snapTimes[len(snapTimes)-1]
}

// rangeResolution is the number of steps to take in the rangeTimes iterator.
const rangeResolution = 250

// rangeTimes returns an iterator that yields times in the given range.
func rangeTimes(start, end time.Time) func(yield func(time.Time) bool) {
	d := end.Sub(start)
	d = d / rangeResolution
	d = snapDownDuration(d)
	t := start.Truncate(d)

	return func(yield func(time.Time) bool) {
		for t.Before(end) {
			if !yield(t) {
				return
			}
			t = t.Add(d)
		}
		// yield once more, to ensure the end time is included
		yield(t)
	}
}
