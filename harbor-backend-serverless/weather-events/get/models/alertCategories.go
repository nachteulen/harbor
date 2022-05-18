package models

type AlertData struct {
	Text     string
	Level    AlertLevel
	Category AlertCategory
}

type AlertLevel string

const (
	CLEAR     AlertLevel = "CLEAR"
	AWARE     AlertLevel = "AWARE"
	WATCH     AlertLevel = "ON WATCH"
	WARNING   AlertLevel = "WARNING"
	DANGEROUS AlertLevel = "DANGEROUS"
)

type AlertCategory string

const (
	EARTHQUAKE   AlertCategory = "Earthquake"
	FLOODS       AlertCategory = "Floods"
	HEATWAVES    AlertCategory = "Heatwaves"
	HURRICANES   AlertCategory = "Hurricanes"
	NONE         AlertCategory = "NONE"
	TORNADOES    AlertCategory = "Tornadoes"
	TSUNAMIS     AlertCategory = "Tsunamis"
	VOLCANO      AlertCategory = "Volcano"
	WILDFIRE     AlertCategory = "Wildfire"
	WINTERSTORMS AlertCategory = "Winter Storms"
)

var AlertCodeToCategorization = map[string]AlertData{
	// WEATHER-RELATED EVENTS
	"BZW":  {Text: "Blizzard Warning", Level: WARNING, Category: WINTERSTORMS},
	"CFA":  {Text: "Coastal Flood Watch", Level: WATCH, Category: FLOODS},
	"CFW":  {Text: "Coastal Flood Warning", Level: WARNING, Category: FLOODS},
	"DSW":  {Text: "Dust Storm Warning", Level: WARNING, Category: NONE},
	"EHA":  {Text: "Extreme Heat Watch", Level: WATCH, Category: HEATWAVES},
	"EHW":  {Text: "Extreme Heat Warning", Level: WARNING, Category: HEATWAVES},
	"EWW":  {Text: "Extreme Wind Warning", Level: WARNING, Category: NONE},
	"FAY":  {Text: "Flash Advisory", Level: AWARE, Category: FLOODS},
	"FFA":  {Text: "Flash Flood Watch", Level: WATCH, Category: FLOODS},
	"FFW":  {Text: "Flash Flood Warning", Level: WARNING, Category: FLOODS},
	"FFS":  {Text: "Flash Flood Statement", Level: AWARE, Category: FLOODS},
	"FLA":  {Text: "Flood Watch", Level: WATCH, Category: FLOODS},
	"FLW":  {Text: "Flood Warning", Level: WARNING, Category: FLOODS},
	"FLS":  {Text: "Flood Statement", Level: AWARE, Category: FLOODS},
	"HWA":  {Text: "High Wind Watch", Level: WATCH, Category: NONE},
	"HWW":  {Text: "High Wind Warning", Level: WARNING, Category: NONE},
	"HUA":  {Text: "Hurricane Watch", Level: WATCH, Category: HURRICANES},
	"HUW":  {Text: "Hurricane Warning", Level: WARNING, Category: HURRICANES},
	"HLS":  {Text: "Hurricane Statement", Level: AWARE, Category: HURRICANES},
	"HTY":  {Text: "Heat Advisory", Level: AWARE, Category: HEATWAVES},
	"MWS":  {Text: "Marine Weather Statement", Level: AWARE, Category: NONE},
	"SVA":  {Text: "Severe Thunderstorm Watch", Level: WATCH, Category: NONE},
	"SVR":  {Text: "Severe Thunderstorm Warning", Level: WARNING, Category: NONE},
	"SVS":  {Text: "Severe Weather Statement", Level: AWARE, Category: NONE},
	"SQW2": {Text: "Snow Squall Warning", Level: WARNING, Category: WINTERSTORMS},
	"SMW":  {Text: "Special Marine Warning", Level: WARNING, Category: NONE},
	"SPS":  {Text: "Special Weather Statement", Level: AWARE, Category: NONE},
	"SSA":  {Text: "Storm Surge Watch", Level: WATCH, Category: NONE},
	"SSW":  {Text: "Storm Surge Warning", Level: WARNING, Category: NONE},
	"TOA":  {Text: "Tornado Watch", Level: WATCH, Category: TORNADOES},
	"TOR":  {Text: "Tornado Warning", Level: WARNING, Category: TORNADOES},
	"TRA":  {Text: "Tropical Storm Watch", Level: WATCH, Category: HURRICANES},
	"TRW":  {Text: "Tropical Storm Warning", Level: WARNING, Category: HURRICANES},
	"TSA":  {Text: "Tsunami Watch", Level: WATCH, Category: TSUNAMIS},
	"TSW":  {Text: "Tsunami Warning", Level: WARNING, Category: TSUNAMIS},
	"WSA":  {Text: "Winter Storm Watch", Level: WATCH, Category: WINTERSTORMS},
	"WSW":  {Text: "Winter Storm Warning", Level: WARNING, Category: WINTERSTORMS},
	"WIY":  {Text: "Wind Advisory", Level: AWARE, Category: NONE},
	// NON-WEATHER RELATED EVENTS
	"AVA":  {Text: "Avalanche Watch", Level: WATCH, Category: NONE},
	"AVW":  {Text: "Avalanche Warning", Level: WARNING, Category: NONE},
	"BHS":  {Text: "Beach Hazards Statement", Level: WATCH, Category: NONE},
	"BLU":  {Text: "Blue Alert", Level: AWARE, Category: NONE},
	"BWY":  {Text: "Brisk Wind Advisory", Level: AWARE, Category: NONE},
	"CAE":  {Text: "Child Abduction Emergency", Level: AWARE, Category: NONE},
	"CDW":  {Text: "Civil Danger Warning", Level: AWARE, Category: NONE},
	"CEM":  {Text: "Civil Emergency Message", Level: WARNING, Category: NONE},
	"CFS":  {Text: "Coastal Flood Statement", Level: AWARE, Category: FLOODS},
	"CFY":  {Text: "Coastal Flood Advisory", Level: WATCH, Category: FLOODS},
	"DSY":  {Text: "Dust Advisory", Level: AWARE, Category: NONE},
	"EQW":  {Text: "Earthquake Warning", Level: WARNING, Category: EARTHQUAKE},
	"ESF":  {Text: "Hydrologic Outlook", Level: AWARE, Category: NONE},
	"EVI":  {Text: "Evacuation Immediate", Level: DANGEROUS, Category: NONE},
	"FGY":  {Text: "Dense Fog Advisory", Level: AWARE, Category: NONE},
	"FLY":  {Text: "Flood Advisory", Level: AWARE, Category: FLOODS},
	"FRW":  {Text: "Fire Warning", Level: WARNING, Category: WILDFIRE},
	"FWA":  {Text: "Fire Weather Watch", Level: WATCH, Category: WILDFIRE},
	"FWW":  {Text: "Red Flag Warning (Fire)", Level: WARNING, Category: WILDFIRE},
	"GLA":  {Text: "Gale Watch", Level: WATCH, Category: NONE},
	"GLW":  {Text: "Gale Warning", Level: WARNING, Category: NONE},
	"HMW":  {Text: "Hazardous Materials Warning", Level: WARNING, Category: NONE},
	"LEW":  {Text: "Law Enforcement Warning", Level: WARNING, Category: NONE},
	"LAE":  {Text: "Local Area Emergency", Level: WARNING, Category: NONE},
	"MAW":  {Text: "Special Marine Warning", Level: WARNING, Category: NONE},
	"MFY":  {Text: "Dense Fog Advisory", Level: AWARE, Category: NONE},
	"TOE":  {Text: "911 Telephone Outage Emergency", Level: DANGEROUS, Category: NONE},
	"NUW":  {Text: "Nuclear Power Plant Warning", Level: WARNING, Category: NONE},
	"RPS":  {Text: "Rip Current Statement", Level: AWARE, Category: NONE},
	"RHW":  {Text: "Radiological Hazard Warning", Level: WARNING, Category: NONE},
	"SCY":  {Text: "Small Craft Advisory", Level: WATCH, Category: NONE},
	"SEW":  {Text: "Hazardous Seas Warning", Level: WARNING, Category: NONE},
	"SPW":  {Text: "Shelter in Place Warning", Level: WARNING, Category: NONE},
	"SUY":  {Text: "High Surf Advisory", Level: AWARE, Category: NONE},
	"VOW":  {Text: "Volcano Warning", Level: WARNING, Category: VOLCANO},
	"NULL": {Text: "Placeholder", Level: CLEAR, Category: NONE},
}
