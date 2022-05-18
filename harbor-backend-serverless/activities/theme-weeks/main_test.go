package main

import (
	"testing"
)

var numWeeks = 11

func TestHandler(t *testing.T) {
	t.Run("negative days elapsed is first week", func(t *testing.T) {
		r, isFirstCycle := getWeekIdx(-1, numWeeks)
		if r != 0 {
			t.Fatalf("expected 0, got %d\n", r)
		}
		if isFirstCycle != true {
			t.Fatalf("expected true - got %t\n", isFirstCycle)
		}
	})

	t.Run("zero days elapsed is first week", func(t *testing.T) {
		r, isFirstCycle := getWeekIdx(0, numWeeks)
		if r != 0 {
			t.Fatalf("expected 0, got %d\n", r)
		}
		if isFirstCycle != true {
			t.Fatalf("expected true - got %t\n", isFirstCycle)
		}
	})

	t.Run("1 day elapsed is second week", func(t *testing.T) {
		r, isFirstCycle := getWeekIdx(1, numWeeks)
		if r != 1 {
			t.Fatalf("expected 1, got %d\n", r)
		}
		if isFirstCycle != true {
			t.Fatalf("expected true - got %t\n", isFirstCycle)
		}
	})

	t.Run("7 days elapsed is still second week", func(t *testing.T) {
		r, isFirstCycle := getWeekIdx(7, numWeeks)
		if r != 1 {
			t.Fatalf("expected 1, got %d\n", r)
		}
		if isFirstCycle != true {
			t.Fatalf("expected true - got %t\n", isFirstCycle)
		}
	})

	t.Run("8 days elapsed is third week", func(t *testing.T) {
		r, isFirstCycle := getWeekIdx(8, numWeeks)
		if r != 2 {
			t.Fatalf("expected 2, got %d\n", r)
		}
		if isFirstCycle != true {
			t.Fatalf("expected true - got %t\n", isFirstCycle)
		}
	})

	t.Run("64 days elapsed marks beginning of the final week", func(t *testing.T) {
		r, isFirstCycle := getWeekIdx(64, numWeeks)
		if r != 10 {
			t.Fatalf("expected 10, got %d\n", r)
		}
		if isFirstCycle != true {
			t.Fatalf("expected true - got %t\n", isFirstCycle)
		}
	})

	t.Run("70 days elapsed marks end of the final week", func(t *testing.T) {
		r, isFirstCycle := getWeekIdx(70, numWeeks)
		if r != 10 {
			t.Fatalf("expected 1, got %d\n", r)
		}
		if isFirstCycle != true {
			t.Fatalf("expected true - got %t\n", isFirstCycle)
		}
	})

	t.Run("71 days restarts the first cycle", func(t *testing.T) {
		r, isFirstCycle := getWeekIdx(71, numWeeks)
		if r != 0 {
			t.Fatalf("expected 1, got %d\n", r)
		}
		if isFirstCycle != false {
			t.Fatalf("expected false - got %t\n", isFirstCycle)
		}
	})

	t.Run("147 days elapsed marks end of the final week, again", func(t *testing.T) {
		r, isFirstCycle := getWeekIdx(147, numWeeks)
		if r != 10 {
			t.Fatalf("expected 1, got %d\n", r)
		}
		if isFirstCycle != false {
			t.Fatalf("expected false - got %t\n", isFirstCycle)
		}
	})

	t.Run("148 days restarts the first cycle, again", func(t *testing.T) {
		r, isFirstCycle := getWeekIdx(148, numWeeks)
		if r != 0 {
			t.Fatalf("expected 1, got %d\n", r)
		}
		if isFirstCycle != false {
			t.Fatalf("expected false - got %t\n", isFirstCycle)
		}
	})

	t.Run("if first cycle, the current week is returned", func(t *testing.T) {
		themes := []*Theme{&Theme{Name: "Current"}}
		theme, _, _ := parseThemes(0, themes, true)
		if theme.Name != "Current" {
			t.Fatalf("expected Current, got %s\n", theme.Name)
		}
	})

	t.Run("if first cycle, the current completed week is returned", func(t *testing.T) {
		themes := []*Theme{&Theme{Name: "Current", Completed: true}}
		theme, _, _ := parseThemes(0, themes, true)
		if theme.Name != "Current" {
			t.Fatalf("expected Current, got %s\n", theme.Name)
		}
	})

	t.Run("if not first cycle, the current week is returned", func(t *testing.T) {
		themes := []*Theme{&Theme{Name: "Current"}}
		theme, _, _ := parseThemes(0, themes, false)
		if theme.Name != "Current" {
			t.Fatalf("expected Current, got %s\n", theme.Name)
		}
	})

	t.Run("if not first cycle, the current completed week is not returned", func(t *testing.T) {
		themes := []*Theme{
			&Theme{Name: "Current", Completed: true},
			&Theme{Name: "Next", Completed: false},
		}
		theme, _, _ := parseThemes(0, themes, false)
		if theme.Name != "Next" {
			t.Fatalf("expected Next, got %s\n", theme.Name)
		}
	})

	t.Run("we restart at incomplete week", func(t *testing.T) {
		themes := []*Theme{
			&Theme{Name: "Next", Completed: false},
			&Theme{Name: "Current", Completed: true},
			&Theme{Name: "Next Comp", Completed: true},
		}
		theme, _, _ := parseThemes(1, themes, false)
		if theme.Name != "Next" {
			t.Fatalf("expected Next, got %s\n", theme.Name)
		}
	})

	t.Run("we restart at incomplete week, end of cycle", func(t *testing.T) {
		themes := []*Theme{
			&Theme{Name: "Next", Completed: false},
			&Theme{Name: "Prev Comp", Completed: true},
			&Theme{Name: "Current", Completed: true},
		}
		theme, _, _ := parseThemes(2, themes, false)
		if theme.Name != "Next" {
			t.Fatalf("expected Next, got %s\n", theme.Name)
		}
	})

	t.Run("we loop around to incomplete week", func(t *testing.T) {
		themes := []*Theme{
			&Theme{Name: "Next Comp", Completed: true},
			&Theme{Name: "Next", Completed: false},
			&Theme{Name: "Current", Completed: true},
		}
		theme, _, _ := parseThemes(2, themes, false)
		if theme.Name != "Next" {
			t.Fatalf("expected Next, got %s\n", theme.Name)
		}
	})

	t.Run("we still get a week if all are completed", func(t *testing.T) {
		themes := []*Theme{&Theme{Completed: true}}
		theme, _, _ := parseThemes(0, themes, false)
		if theme == nil {
			t.Fatal("expected a theme, got nil")
		}
	})

	t.Run("we still get a week if all are completed, multiple even", func(t *testing.T) {
		themes := []*Theme{
			&Theme{Completed: true},
			&Theme{Completed: true},
		}
		theme, _, _ := parseThemes(0, themes, false)
		if theme == nil {
			t.Fatal("expected a theme, got nil")
		}
	})

	t.Run("we still get a week if all are completed, multiple odd", func(t *testing.T) {
		themes := []*Theme{
			&Theme{Completed: true},
			&Theme{Completed: true},
			&Theme{Completed: true},
		}
		theme, _, _ := parseThemes(0, themes, false)
		if theme == nil {
			t.Fatal("expected a theme, got nil")
		}
	})
}
