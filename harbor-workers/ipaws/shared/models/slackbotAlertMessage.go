package models

type SlackRequestBody struct {
	Blocks []Block `json:"blocks"`
}

type Block struct {
	Type     string  `json:"type"`
	Sections Section `json:"text"`
}

type Section struct {
	Type string `json:"type"`
	Text string `json:"text"`
	Emoji bool `json:"emoji,omitempty"`
}

type CityData struct {
	Name string
	URL string
	Lat float64
	Long float64
}

type UserRow struct {
	Email     *string `db:"email"`
	Address   *string `db:"address"`
	Latitude  *string `db:"latitude"`
	Longitude *string `db:"longitude"`
}

type User struct {
	Email string
	Address string
	Latitude string
	Longitude string
}