package models

type WorkflowTriggerAlert struct {
	Name       string       `json:"eventName"`
	Email      string       `json:"email"`
	DataFields BeaconFields `json:"dataFields""`
}

type BeaconFields struct {
	PushData BeaconPush `json:"beaconPush"`
}

type BeaconPush struct {
	Update               bool   `json:"beaconUpdate"`
	AlertId              string `json:"beaconAlertId"`
	PreviousOutlookLevel string `json:"beaconPreviousOutlookLevel"`
	CurrentOutlookLevel  string `json:"beaconCurrentOutlookLevel"`
	AlertType            string `json:"beaconAlertType"`
	WeatherEvent         string `json:"beaconWeatherEvent"`
	Deeplink             string `json:"beaconDeeplink"`
}

var OutlookLevelDict = map[string]string{
	"CLEAR":     "0",
	"AWARE":     "1",
	"ON WATCH":  "2",
	"WARNING":   "3",
	"DANGEROUS": "4",
}
