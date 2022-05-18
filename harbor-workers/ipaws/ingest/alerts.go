package main

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/service/sns"
	"github.com/helloharbor/harbor-workers/ipaws/shared/models"

	geo "github.com/helloharbor/harbor-workers/ipaws/shared/geometries"
	log "github.com/sirupsen/logrus"
)

var (
	redisQueueKey      = os.Getenv("REDIS_ALERT_QUEUE_KEY")
	redisQueueMaxDepth = os.Getenv("ALERT_QUEUE_MAX_DEPTH")
)

func getAlerts(body []byte) (*models.AlertsMsg, error) {
	alerts := models.AlertsXML{}
	err := xml.Unmarshal(body, &alerts)
	if err != nil {
		return nil, err
	}
	parsedAlerts, err := parseAlerts(alerts)
	if err != nil {
		return nil, err
	}
	_, err = json.Marshal(*parsedAlerts)
	if err != nil {
		return nil, err
	}

	return parsedAlerts, nil
}

func checkCacheForAlert(identifier string) (bool, error) {
	exists, err := redisConn.Exists(ctx, identifier).Result()
	if err != nil {
		return false, err
	} else if exists == 1 {
		return true, nil
	}
	return false, nil
}

func writeToAlertQueue(alert models.ShortAlertMsg) error {
	strAlert, err := json.Marshal(alert)
	if err != nil {
		return err
	}

	// get queue length
	qLen, err := redisConn.LLen(ctx, redisQueueKey).Result()
	if err != nil {
		return fmt.Errorf("failed to get queue length: %s", err)
	}
	// if at capacity pop oldest off queue
	maxQLen, err := strconv.ParseInt(redisQueueMaxDepth, 10, 64)
	if err != nil {
		return err
	}
	if qLen >= maxQLen {
		redisConn.RPop(ctx, redisQueueKey)
	}
	// push alert onto queue
	_, err = redisConn.LPush(ctx, redisQueueKey, strAlert).Result()
	if err != nil {
		return fmt.Errorf("failed adding alert to queue: %s", err)
	}

	return nil
}

func cacheAlert(alert models.AlertMsg) error {
	alertMsg, err := json.Marshal(alert)
	if err != nil {
		return err
	}

	oneDay := time.Hour * 24
	if err := redisConn.Set(ctx, alert.Identifier, alertMsg, oneDay).Err(); err != nil {
		return err
	}

	return nil
}

func getShortFormAlerts(alert models.AlertMsg) ([]*models.ShortAlertMsg, error) {
	alerts := []*models.ShortAlertMsg{}

	polygonStr := alert.Info.Area.Polygon
	geocodeArr := alert.Info.Area.Geocodes
	if len(polygonStr) != 0 {
		bbStr, err := computeBoundingBox(polygonStr)
		if err != nil {
			return nil, fmt.Errorf("failed getting bounding box from polygon string: %s", err)
		}
		sfa, err := createShortFormAlert(alert, polygonStr, *bbStr)
		if err != nil {
			return nil, fmt.Errorf("failed to create short form alert: %s", err)
		}
		alerts = append(alerts, sfa)
	} else if len(geocodeArr) != 0 {
		for _, gc := range geocodeArr {
			var rows []*GeocodeUgcRow
			gcStr := fmt.Sprintf("%s%s", gc[0:2], gc[3:])
			if err := pgDB.Select(&rows, ugcQuery, gcStr); err != nil {
				return nil, fmt.Errorf("geocode_ugc query failed: %s", err)
			}
			for _, row := range rows {
				bbStr := fmt.Sprintf("%f %f %f %f",
					row.BBLatLo, row.BBLatHi, row.BBLngLo, row.BBLngHi)
				sfa, err := createShortFormAlert(alert, row.ConvexHull, bbStr)
				if err != nil {
					return nil, fmt.Errorf("failed to create short form alert: %s", err)
				}
				alerts = append(alerts, sfa)
			}
		}
	} else {
		log.WithFields(stdFields).Warn("alert has not geometries polygon/geocode")
		return nil, nil
	}

	return alerts, nil
}

func createShortFormAlert(alert models.AlertMsg, polygon string, boundingBox string) (*models.ShortAlertMsg, error){
	eventCode := alert.Info.EventCode.Value
	codeMapping, ok := models.AlertCodeToCategorization[alert.Info.EventCode.Value]
	if !ok {
		return nil, fmt.Errorf("code %s does not exists in category mapping", eventCode)
	}

	sfMsgOjb := &models.ShortAlertMsg{
		Identifier: alert.Identifier,
		IsUpdate: strings.ToLower(alert.MsgType) == "update",
		RefIds: alert.References,
		Polygon: polygon,
		BoundingBox: boundingBox,
		Categorization: models.AlertCategorization{
			Text: codeMapping.Text,
			Category: string(codeMapping.Category),
			Code: alert.Info.EventCode.Value,
			Level: string(codeMapping.Level),
		},
		OnsetTime: alert.Info.Onset,
		ExpirationTime: alert.Info.Expires,
	}

	return sfMsgOjb, nil
}

func computeBoundingBox(polygon string) (*string, error) {
	rRect, err := geo.GetBoundingBoxFromPolygonString(polygon)
	if err != nil {
		return nil, err
	}
	rRect.ScaleBoundingBox(16.0)

	bbStr := fmt.Sprintf("%f %f %f %f",
		rRect.LatLo, rRect.LatHi,
		rRect.LngLo, rRect.LngHi)

	return &bbStr, nil
}

func sendShortFormAlert(alert models.ShortAlertMsg) error {
	sfMsg, err := json.Marshal(alert)
	if err != nil {
		return err
	}
	sfMsgStr := string(sfMsg)

	_, err = snsClient.Publish(&sns.PublishInput{
		Message:  &sfMsgStr,
		TopicArn: &snsArn,
	})
	if err != nil {
		return err
	}

	return nil
}

func parseAlerts(alerts models.AlertsXML) (*models.AlertsMsg, error) {
	parsedAlerts := models.AlertsMsg{}
	for _, alert := range alerts.Alert {
		parsedAlert := models.AlertMsg{}
		// check alert type and remove urn
		strippedId := strings.Split(alert.Identifier, "urn:oid:")
		if len(strippedId) != 2 {
			continue
		}

		parsedAlert.Identifier = strippedId[1]
		parsedAlert.Status = alert.Status
		parsedAlert.MsgType = alert.MsgType
		parsedAlert.Scope = alert.Scope
		parsedAlert.References = parseReferences(alert.References)

		parsedAlert.Info.Category = alert.Info.Category
		parsedAlert.Info.Event = alert.Info.Event
		parsedAlert.Info.ResponseType = alert.Info.ResponseType
		parsedAlert.Info.Urgency = alert.Info.Urgency
		parsedAlert.Info.Severity = alert.Info.Severity
		parsedAlert.Info.Certainty = alert.Info.Certainty

		eventCode := alert.Info.EventCode[0]
		if eventCode.Value == "NWS" {
			eventCode = alert.Info.EventCode[1]
		}

		parsedAlert.Info.EventCode = models.EventCodeMsg{
			ValueName: eventCode.ValueName,
			Value: eventCode.Value,
		}

		var tErr error
		parsedAlert.Info.Effective, tErr = getUTC3339(alert.Info.Effective)
		if tErr != nil {
			return nil, fmt.Errorf("failed to convert onset event time (%s)", alert.Info.Effective)
		}
		parsedAlert.Info.Onset, tErr = getUTC3339(alert.Info.Onset)
		if tErr != nil {
			return nil, fmt.Errorf("failed to convert onset event time (%s)", alert.Info.Onset)
		}
		parsedAlert.Info.Expires, tErr = getUTC3339(alert.Info.Expires)
		if tErr != nil {
			return nil, fmt.Errorf("failed to convert onset event time (%s)", alert.Info.Expires)
		}
		parsedAlert.Info.Headline = alert.Info.Headline
		parsedAlert.Info.Description = alert.Info.Description
		parsedAlert.Info.Instruction = alert.Info.Instruction
		parsedAlert.Info.Area.AreaDesc = alert.Info.Area.AreaDesc
		parsedAlert.Info.Area.Polygon = alert.Info.Area.Polygon
		parsedAlert.Info.Area.Circle = alert.Info.Area.Circle

		parsedAlert.Info.Area.Geocodes = []string{}
		for _, gc := range alert.Info.Area.Geocode {
			if gc.ValueName == "UGC" {
				parsedAlert.Info.Area.Geocodes = append(parsedAlert.Info.Area.Geocodes, gc.Value)
			}
		}

		parsedAlerts.Alert = append(parsedAlerts.Alert, parsedAlert)
	}

	return &parsedAlerts, nil
}

func parseReferences(refs string) []string {
	var newRefsArray []string
	refsArray := strings.Split(refs, ",")
	for _, ref := range refsArray {
		if strings.Contains(ref, "urn:oid:") {
			strippedRef := strings.Split(ref, "urn:oid:")
			newRefsArray = append(newRefsArray, strippedRef[1])
		}
	}

	return newRefsArray
}

func getUTC3339(alertTimeMsg string) (string, error) {
	isoTime, err := time.Parse(time.RFC3339, alertTimeMsg)
	if err != nil {
		return "", err
	}
	formatted := isoTime.UTC().Format("2006-01-02T15:04:05.000Z0700")

	return formatted, nil
}
