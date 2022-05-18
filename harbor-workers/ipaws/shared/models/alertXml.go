package models

type AlertsXML struct {
	Alert []AlertXML `xml:"alert"`
}

type AlertXML struct {
	Identifier string `xml:"identifier"`
	Status     string `xml:"status"`
	MsgType    string `xml:"msgType"`
	Scope      string `xml:"scope"`
	References string `xml:"references"`
	Info       struct {
		Category     string `xml:"category"`
		Event        string `xml:"event"`
		ResponseType string `xml:"responseType"`
		Urgency      string `xml:"urgency"`
		Severity     string `xml:"severity"`
		Certainty    string `xml:"certainty"`
		EventCode    []struct {
			ValueName string `xml:"valueName"`
			Value     string `xml:"value"`
		} `xml:"eventCode"`
		Effective   string `xml:"effective"`
		Onset       string `xml:"onset"`
		Expires     string `xml:"expires"`
		Headline    string `xml:"headline"`
		Description string `xml:"description"`
		Instruction string `xml:"instruction"`
		Area        struct {
			AreaDesc string `xml:"areaDesc"`
			Polygon  string `xml:"polygon"`
			Circle   string `xml:"circle"`
			Geocode  []struct {
				ValueName string `xml:"valueName"`
				Value     string `xml:"value"`
			} `xml:"geocode"`
		} `xml:"area"`
	} `xml:"info"`
}