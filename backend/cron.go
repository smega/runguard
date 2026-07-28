// cron.go — dependency-free cron expression support.
//
// Jobs can be monitored two ways: a plain "expect a heartbeat every N
// minutes" interval (the original and still-default model), or a standard
// 5-field cron expression ("0 3 * * *") for jobs that run on a fixed
// schedule rather than a fixed cadence — the wrong model for "once a day at
// 3am" was forcing users to approximate it as "interval: 1440 minutes",
// which drifts if the job runs a few minutes late one day.
//
// There's no Go module proxy access in this build environment, so this is a
// small hand-rolled parser rather than a pulled-in dependency. It supports
// the common syntax: "*", "*/step", "a-b", "a-b/step", and comma-separated
// lists of any of those, same as a standard crontab line. Times are
// evaluated in UTC only for now — no per-job timezone support yet.
package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// CronSchedule is a parsed 5-field cron expression (minute hour
// day-of-month month day-of-week).
type CronSchedule struct {
	minute [60]bool
	hour   [24]bool
	dom    [32]bool // 1..31 used, 0 unused
	month  [13]bool // 1..12 used, 0 unused
	dow    [7]bool  // 0..6, 0 = Sunday

	// domStar/dowStar record whether those two fields were "*" in the
	// original expression. When both day-of-month and day-of-week are
	// restricted (neither is "*"), standard cron semantics OR them
	// together rather than AND — e.g. "0 0 1,15 * MON" means the 1st,
	// the 15th, OR any Monday, not "the 1st/15th AND a Monday".
	domStar bool
	dowStar bool
}

// ParseCronSchedule parses a standard 5-field cron expression.
func ParseCronSchedule(expr string) (*CronSchedule, error) {
	fields := strings.Fields(strings.TrimSpace(expr))
	if len(fields) != 5 {
		return nil, fmt.Errorf("expected 5 fields (minute hour day month weekday), got %d", len(fields))
	}

	s := &CronSchedule{}

	if err := parseCronField(fields[0], 0, 59, s.minute[:]); err != nil {
		return nil, fmt.Errorf("minute field: %w", err)
	}
	if err := parseCronField(fields[1], 0, 23, s.hour[:]); err != nil {
		return nil, fmt.Errorf("hour field: %w", err)
	}
	if err := parseCronField(fields[2], 1, 31, s.dom[:]); err != nil {
		return nil, fmt.Errorf("day-of-month field: %w", err)
	}
	if err := parseCronField(fields[3], 1, 12, s.month[:]); err != nil {
		return nil, fmt.Errorf("month field: %w", err)
	}
	if err := parseCronField(fields[4], 0, 6, s.dow[:]); err != nil {
		return nil, fmt.Errorf("day-of-week field: %w", err)
	}

	s.domStar = strings.TrimSpace(fields[2]) == "*"
	s.dowStar = strings.TrimSpace(fields[4]) == "*"

	return s, nil
}

// parseCronField parses one comma-separated cron field ("*", "*/2", "1-5",
// "1-5/2", "3", or a mix via commas) and sets the matching slots in out.
// out must be pre-sized so index min..max are valid (e.g. len 60 for
// minutes 0..59).
func parseCronField(field string, min, max int, out []bool) error {
	for _, part := range strings.Split(field, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		step := 1
		rangePart := part
		if idx := strings.Index(part, "/"); idx >= 0 {
			rangePart = part[:idx]
			n, err := strconv.Atoi(part[idx+1:])
			if err != nil || n <= 0 {
				return fmt.Errorf("invalid step in %q", part)
			}
			step = n
		}

		var lo, hi int
		switch {
		case rangePart == "*":
			lo, hi = min, max
		case strings.Contains(rangePart, "-"):
			bounds := strings.SplitN(rangePart, "-", 2)
			a, err1 := strconv.Atoi(strings.TrimSpace(bounds[0]))
			b, err2 := strconv.Atoi(strings.TrimSpace(bounds[1]))
			if err1 != nil || err2 != nil {
				return fmt.Errorf("invalid range in %q", part)
			}
			lo, hi = a, b
		default:
			n, err := strconv.Atoi(rangePart)
			if err != nil {
				return fmt.Errorf("invalid value %q", part)
			}
			lo, hi = n, n
		}

		if lo < min || hi > max || lo > hi {
			return fmt.Errorf("value out of range in %q (expected %d-%d)", part, min, max)
		}

		for v := lo; v <= hi; v += step {
			out[v] = true
		}
	}
	return nil
}

// Matches reports whether t (evaluated in UTC) matches the schedule.
func (s *CronSchedule) Matches(t time.Time) bool {
	t = t.UTC()
	if !s.minute[t.Minute()] {
		return false
	}
	if !s.hour[t.Hour()] {
		return false
	}
	if !s.month[int(t.Month())] {
		return false
	}

	domMatch := s.dom[t.Day()]
	dowMatch := s.dow[int(t.Weekday())]

	switch {
	case s.domStar && s.dowStar:
		return true
	case s.domStar:
		return dowMatch
	case s.dowStar:
		return domMatch
	default:
		return domMatch || dowMatch
	}
}

// maxCronSearchMinutes bounds how far into the future NextAfter will look
// before giving up. Two years is comfortably longer than any realistic
// monitoring cadence, while still catching genuinely unsatisfiable
// expressions (e.g. day-of-month 31 combined with a month field that
// excludes every 31-day month) instead of looping forever.
const maxCronSearchMinutes = 2 * 366 * 24 * 60

// NextAfter returns the next time strictly after t that matches the
// schedule, minute-granularity (seconds/nanoseconds are ignored, matching
// how heartbeats are actually timestamped).
func (s *CronSchedule) NextAfter(t time.Time) (time.Time, error) {
	next := t.UTC().Truncate(time.Minute).Add(time.Minute)
	for i := 0; i < maxCronSearchMinutes; i++ {
		if s.Matches(next) {
			return next, nil
		}
		next = next.Add(time.Minute)
	}
	return time.Time{}, fmt.Errorf("no matching time found within the search window — check for an unsatisfiable cron expression")
}

// jobLateness determines whether a job is currently late and by how much,
// handling both plain interval+grace jobs and cron-scheduled jobs the same
// way. ok=false means the job has never reported a heartbeat at all (the
// caller should treat that as "never_seen", not "late").
func jobLateness(j *Job, now time.Time) (late bool, lateBy time.Duration, ok bool) {
	if j.LastHeartbeatAt == nil || j.LastHeartbeatAt.IsZero() {
		return false, 0, false
	}
	grace := time.Duration(j.GraceSecs) * time.Second

	if expr := strings.TrimSpace(j.CronExpr); expr != "" {
		if sched, err := ParseCronSchedule(expr); err == nil {
			if expectedNext, err := sched.NextAfter(*j.LastHeartbeatAt); err == nil {
				deadline := expectedNext.Add(grace)
				if now.After(deadline) {
					return true, now.Sub(deadline), true
				}
				return false, 0, true
			}
		}
		// Invalid or unsatisfiable cron expression (shouldn't happen since
		// it's validated on save) — fall through to interval mode as a
		// safety net rather than silently never alerting on this job.
	}

	threshold := time.Duration(j.ExpectedIntervalSecs)*time.Second + grace
	elapsed := now.Sub(*j.LastHeartbeatAt)
	if elapsed > threshold {
		return true, elapsed - threshold, true
	}
	return false, 0, true
}
