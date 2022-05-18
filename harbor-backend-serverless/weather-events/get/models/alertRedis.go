package models

type RedisAlertMsg struct {
	Identifier string       `json:"identifier"`
	Status     string       `json:"status"`
	MsgType    string       `json:"msgType"`
	Scope      string       `json:"scope"`
	References []string     `json:"references"`
	Info       RedisInfoMsg `json:"info"`
}

type RedisInfoMsg struct {
	Category     string              `json:"category"`
	Event        string              `json:"event"`
	ResponseType string              `json:"responseType"`
	Urgency      string              `json:"urgency"`
	Severity     string              `json:"severity"`
	Certainty    string              `json:"certainty"`
	EventCode    RedisEventCodeMsg `json:"eventCode"`
	Effective    string              `json:"effective"`
	Onset        string              `json:"onset"`
	Expires      string              `json:"expires"`
	Headline     string              `json:"headline"`
	Description  string              `json:"description"`
	Instruction  string              `json:"instruction"`
	Area         RedisAreaMsg        `json:"area"`
}

type RedisEventCodeMsg struct {
	ValueName string `xml:"valueName"`
	Value     string `xml:"value"`
}

type RedisAreaMsg struct {
	AreaDesc string   `json:"areaDesc"`
	Polygon  string   `json:"polygon"`
	Circle   string   `json:"circle""`
	Geocodes []string `json:"geocodes"`
}
