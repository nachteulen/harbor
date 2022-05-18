package models

type ShortAlertMsg struct {
	Identifier     string              `json:"identifier"`
	IsUpdate       bool                `json:"isUpdate"`
	RefIds         []string            `json:"referenceIDs"`
	Categorization AlertCategorization `json:"categorization"`
	BoundingBox    string              `json:"boundingBox"`
	Polygon        string              `json:"polygon"`
	OnsetTime      string              `json:"onsetTime"`
	ExpirationTime string              `json:"expirationTime"`
}

type AlertCategorization struct {
	Text     string `json:"text"`
	Category string `json:"category"`
	Code     string `json:"code"`
	Level    string `json:"level"`
}
