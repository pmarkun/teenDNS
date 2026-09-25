package policy

import (
	"fmt"
	"strings"
	"time"
	_ "time/tzdata"
)

const DefaultTimeZone = "America/Sao_Paulo"

func profileLocation(name string) (*time.Location, error) {
	if name == "" {
		name = DefaultTimeZone
	}
	location, err := time.LoadLocation(name)
	if err != nil {
		return nil, fmt.Errorf("unknown IANA time zone %q", name)
	}
	return location, nil
}

func validateWindow(window TimeWindow) error {
	id := strings.TrimSpace(window.ID)
	if id == "" {
		return fmt.Errorf("id is required")
	}
	if len([]rune(id)) > 80 {
		return fmt.Errorf("id must be at most 80 characters")
	}
	label := strings.TrimSpace(window.Label)
	if label == "" {
		return fmt.Errorf("label is required")
	}
	if len([]rune(label)) > 80 {
		return fmt.Errorf("label must be at most 80 characters")
	}
	start, err := parseMinute(window.Start)
	if err != nil {
		return fmt.Errorf("invalid start time %q", window.Start)
	}
	end, err := parseMinute(window.End)
	if err != nil {
		return fmt.Errorf("invalid end time %q", window.End)
	}
	if start == end {
		return fmt.Errorf("start and end times must differ")
	}
	if len(window.Days) == 0 {
		return fmt.Errorf("at least one weekday is required")
	}
	seen := make(map[int]struct{}, len(window.Days))
	for _, day := range window.Days {
		if day < 0 || day > 6 {
			return fmt.Errorf("weekday %d must be between 0 and 6", day)
		}
		if _, exists := seen[day]; exists {
			return fmt.Errorf("weekday %d is repeated", day)
		}
		seen[day] = struct{}{}
	}
	return nil
}

func parseMinute(value string) (int, error) {
	parsed, err := time.Parse("15:04", value)
	if err != nil || parsed.Format("15:04") != value {
		return 0, fmt.Errorf("time must use HH:MM")
	}
	return parsed.Hour()*60 + parsed.Minute(), nil
}

func windowActive(window TimeWindow, localNow time.Time) bool {
	start, _ := parseMinute(window.Start)
	end, _ := parseMinute(window.End)
	minute := localNow.Hour()*60 + localNow.Minute()
	today := int(localNow.Weekday())
	previousDay := (today + 6) % 7

	for _, day := range window.Days {
		if day == today {
			if end > start && minute >= start && minute < end {
				return true
			}
			if end < start && minute >= start {
				return true
			}
		}
		if end < start && day == previousDay && minute < end {
			return true
		}
	}
	return false
}

func activeScheduledAction(schedules []ScheduledAction, localNow time.Time) (ScheduledAction, bool) {
	for _, schedule := range schedules {
		if windowActive(schedule.TimeWindow, localNow) {
			return schedule, true
		}
	}
	return ScheduledAction{}, false
}

func occupyWindow(occupied map[int]string, window TimeWindow) string {
	start, _ := parseMinute(window.Start)
	end, _ := parseMinute(window.End)
	duration := end - start
	if duration <= 0 {
		duration += 24 * 60
	}
	for _, day := range window.Days {
		first := day*24*60 + start
		for offset := 0; offset < duration; offset++ {
			minute := (first + offset) % (7 * 24 * 60)
			if existing := occupied[minute]; existing != "" {
				return existing
			}
			occupied[minute] = window.ID
		}
	}
	return ""
}
