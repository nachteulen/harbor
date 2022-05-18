package models

type AlertsMsg struct {
	Alert []AlertMsg `json:"alert"`
}

type AlertMsg struct {
	Identifier string   `json:"identifier"`
	Status     string   `json:"status"`
	MsgType    string   `json:"msgType"`
	Scope      string   `json:"scope"`
	References []string `json:"references"`
	Info       InfoMsg  `json:"info"`
}

type InfoMsg struct {
	Category     string       `json:"category"`
	Event        string       `json:"event"`
	ResponseType string       `json:"responseType"`
	Urgency      string       `json:"urgency"`
	Severity     string       `json:"severity"`
	Certainty   string       `json:"certainty"`
	EventCode   EventCodeMsg `json:"eventCode"`
	Effective   string       `json:"effective"`
	Onset        string       `json:"onset"`
	Expires      string       `json:"expires"`
	Headline     string       `json:"headline"`
	Description  string       `json:"description"`
	Instruction string       `json:"instruction"`
	Area        AreaMsg      `json:"area"`
}

type EventCodeMsg struct {
	ValueName string `json:"valueName"`
	Value     string `json:"value"`
}

type AreaMsg struct {
	AreaDesc string   `json:"areaDesc"`
	Polygon  string   `json:"polygon"`
	Circle   string   `json:"circle"`
	Geocodes []string `json:"geocodes"`
}
