// silence_schedule.go
package main

import (
	"fmt"
	"time"
)

// parseHHMM parses a "HH:MM" time-of-day string into minutes since midnight.
func parseHHMM(s string) (int, error) {
	t, err := time.Parse("15:04", s)
	if err != nil {
		return 0, err
	}
	return t.Hour()*60 + t.Minute(), nil
}

// validateSilenceSchedules checks that every schedule has well-formed
// HH:MM times and in-range weekdays, before it's allowed to be saved.
func validateSilenceSchedules(schedules []SilenceSchedule) error {
	for i, s := range schedules {
		if _, err := parseHHMM(s.StartTime); err != nil {
			return fmt.Errorf("schedule %d: invalid start_time %q (want HH:MM)", i, s.StartTime)
		}
		if _, err := parseHHMM(s.EndTime); err != nil {
			return fmt.Errorf("schedule %d: invalid end_time %q (want HH:MM)", i, s.EndTime)
		}
		for _, d := range s.Days {
			if d < 0 || d > 6 {
				return fmt.Errorf("schedule %d: day %d out of range (0=Sunday..6=Saturday)", i, d)
			}
		}
	}
	return nil
}

func scheduleMatchesDay(days []int, weekday int) bool {
	if len(days) == 0 {
		return true
	}
	for _, d := range days {
		if d == weekday {
			return true
		}
	}
	return false
}

// jobIsScheduleSilenced reports whether "now" falls inside one of the job's
// recurring weekly silence windows. Called from jobIsSilenced in jobs.go.
func jobIsScheduleSilenced(j *Job, now time.Time) bool {
	if len(j.SilenceSchedules) == 0 {
		return false
	}
	now = now.UTC()
	nowMinutes := now.Hour()*60 + now.Minute()
	weekday := int(now.Weekday())

	for _, s := range j.SilenceSchedules {
		start, err1 := parseHHMM(s.StartTime)
		end, err2 := parseHHMM(s.EndTime)
		if err1 != nil || err2 != nil || start == end {
			continue
		}

		if start < end {
			// Same-day window (e.g. 06:00-08:00).
			if scheduleMatchesDay(s.Days, weekday) && nowMinutes >= start && nowMinutes < end {
				return true
			}
			continue
		}

		// Wraps past midnight (e.g. 23:00-01:00): active either from
		// StartTime today, or still running into EndTime the morning
		// after a day the rule matched.
		if scheduleMatchesDay(s.Days, weekday) && nowMinutes >= start {
			return true
		}
		prevWeekday := (weekday + 6) % 7
		if scheduleMatchesDay(s.Days, prevWeekday) && nowMinutes < end {
			return true
		}
	}
	return false
}
