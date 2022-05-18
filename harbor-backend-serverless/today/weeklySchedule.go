package main

import (
	"fmt"
)

func sortWeeklySchedule(
	risks map[string]*Risk,
	themes map[string]*Theme,
	schedule []*ScheduleItem,
	weekIdx int,
	userID string,
) []*Week {
	earliestIncompleteIdx := -1
	latestIncompleteIdx := -1
	weeklySchedule := []*Week{}
	orderedWeeklySchedule := []*Week{}

	for i, s := range schedule {
		week := &Week{ID: s.ID, Type: s.Type}

		if s.Type == "theme" {
			t, ok := themes[fmt.Sprintf("%d", s.ID)]
			if !ok {
				fmt.Printf("theme(%d) not found for user(%s)\n", s.ID, userID)
				continue
			}
			week.Name = t.Name
			week.Progress = t.Progress
		} else if s.Type == "risk" {
			r, ok := risks[fmt.Sprintf("%d", s.ID)]
			if !ok {
				fmt.Printf("risk(%d) not found for user(%s)\n", s.ID, userID)
				continue
			}
			week.Name = r.Name
			week.Progress = r.Progress
		} else {
			fmt.Printf("unexpected type(%s) for user(%s) schedule\n", s.Type, userID)
			continue
		}

		if i < weekIdx {
			if earliestIncompleteIdx == -1 && week.Progress != 1 {
				earliestIncompleteIdx = i
			}
		} else {
			if latestIncompleteIdx == -1 && week.Progress != 1 {
				latestIncompleteIdx = i
			}
		}

		weeklySchedule = append(weeklySchedule, week)
		if latestIncompleteIdx != -1 {
			// could also just append to early?
			orderedWeeklySchedule = append(orderedWeeklySchedule, week)
		}
	}

	if len(orderedWeeklySchedule) == 0 {
		if earliestIncompleteIdx != -1 {
			i := earliestIncompleteIdx
			for {
				orderedWeeklySchedule = append(orderedWeeklySchedule, weeklySchedule[i])
				i += 1
				if i == len(weeklySchedule) {
					i = 0
				}
				if i == earliestIncompleteIdx {
					break
				}
			}
		} else {
			// we can determine they are all completed, no reorder necessary
			orderedWeeklySchedule = weeklySchedule
		}
	} else {
		for i := 0; i < latestIncompleteIdx; i++ {
			orderedWeeklySchedule = append(orderedWeeklySchedule, weeklySchedule[i])
		}
	}

	return orderedWeeklySchedule
}
