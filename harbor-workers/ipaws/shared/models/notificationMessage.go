package models

type NotificationMessage struct {
	Identifier     string              `json:"identifier"`
	RefIds         []string            `json:"referenceIDs"`
	Users          []string            `json:"Users"`
	Polygon        string              `json:"polygon"`
	OnsetTime      string              `json:"onsetTime"`
}
