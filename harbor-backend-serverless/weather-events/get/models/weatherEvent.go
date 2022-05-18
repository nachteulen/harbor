package models

type WeatherEventResponse struct {
	ID             string                     `json:"id"`
	RefIds         []string                   `json:"referenceIDs"`
	Status         string                     `json:"status"`
	Categorization WeatherEventCategorization `json:"categorization"`
	Headline       string                     `json:"headline"`
	Description    string                     `json:"description"`
	Instructions   string                     `json:"instructions"`
	Polygon        []WeatherEventCoords       `json:"polygon"`
	OnsetTime      string                     `json:"onsetTime"`
	ExpirationTime string                     `json:"expirationTime"`
}

type WeatherEventCategorization struct {
	Text     string `json:"text"`
	Category string `json:"category"`
	Code     string `json:"code"`
	Level    string `json:"level"`
}

type WeatherEventCoords struct {
	Lat  string `json:"lat"`
	Long string `json:"long"`
}