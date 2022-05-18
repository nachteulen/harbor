package models

type WeatherAlert struct {
	Identifier     string              `json:"identifier"`
	IsUpdate       bool                `json:"isUpdate""`
	RefIds         []string            `json:"referenceIDs"`
	Categorization AlertCategorization `json:"categorization"`
	Polygon        string              `json:"polygon"`
	BoundingBox    string              `json:"boundingBox"`
	OnsetTime      string              `json:"onsetTime"`
	ExpirationTime string              `json:"expirationTime"`
}

type AlertCategorization struct {
	Text     string `json:"text"`
	Category string `json:"category"`
	Code     string `json:"code"`
	Level    string `json:"level"`
}
