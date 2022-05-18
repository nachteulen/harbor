package models

type WeatherEventsResponse struct {
	ID             string                     `json:"id"`
	RefIds         []string                   `json:"referenceIDs"`
	Categorization WeatherEventCategorization `json:"categorization"`
	OnsetTime      string                     `json:"onsetTime"`
	ExpirationTime string                     `json:"expirationTime"`
}

type WeatherEventCategorization struct {
	Text     string `json:"text"`
	Category string `json:"category"`
	Code     string `json:"code"`
	Level    string `json:"level"`
}
