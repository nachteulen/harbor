package main

import (
	"testing"
)

var risks = map[string]*Risk{
	"2": &Risk{Progress: 0.2, Name: "Risk 2"},
	"1": &Risk{Progress: 0.1, Name: "Risk 1"},
}

var themes = map[string]*Theme{
	"4": &Theme{Progress: 0.4, Name: "Theme 4"},
	"3": &Theme{Progress: 0.3, Name: "Theme 3"},
	"5": &Theme{Progress: 0.5, Name: "Theme 5"},
}

var schedule = []*ScheduleItem{
	&ScheduleItem{Type: "risk", ID: 2},
	&ScheduleItem{Type: "risk", ID: 1},
	&ScheduleItem{Type: "theme", ID: 4},
	&ScheduleItem{Type: "theme", ID: 3},
	&ScheduleItem{Type: "theme", ID: 5},
}

func TestHandler(t *testing.T) {
	t.Run("weekIdx schedule item incomplete", func(t *testing.T) {
		expected := "Risk 1"
		result := sortWeeklySchedule(risks, themes, schedule, 1, "some-id")
		if result[0].Name != expected {
			t.Fatalf("expected %s, got %s\n", expected, result[0].Name)
		}
	})

	t.Run("weekIdx schedule item complete, look forward", func(t *testing.T) {
		expected := "Theme 4"
		originalReadiness := risks["1"].Progress
		risks["1"].Progress = 1

		result := sortWeeklySchedule(risks, themes, schedule, 1, "some-id")
		if result[0].Name != expected {
			t.Fatalf("expected %s, got %s\n", expected, result[0].Name)
		}

		risks["1"].Progress = originalReadiness
	})

	t.Run("weekIdx schedule item complete, look backwards", func(t *testing.T) {
		expected := "Risk 2"
		originalProgress := themes["5"].Progress
		themes["5"].Progress = 1

		result := sortWeeklySchedule(risks, themes, schedule, 4, "some-id")
		if result[0].Name != expected {
			t.Fatalf("expected %s, got %s\n", expected, result[0].Name)
		}

		themes["5"].Progress = originalProgress
	})
}
