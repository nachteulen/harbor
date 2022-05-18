package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"
)

type SubscriptionResp struct {
	Type            string `json:"type"`
	Name            string `json:"name"`
	Exp             string `json:"periodEndsAt"`
	Cancellation    string `json:"cancellationRequestedAt"`
	IsTrialPeriod   bool   `json:"isTrialPeriod"`
	isTrialEligible bool   `json:"isTrialEligible"`
}

func (s *SubscriptionResp) GetStatus(userID int64) (string, error) {
	if s.IsTrialPeriod {
		tmplt := "unexpected trial subscription(%+v) for user(%d)"
		return "", fmt.Errorf(tmplt, s, userID)
	} else if !s.isTrialEligible && len(s.Exp) == 0 {
		// TODO: investigate how we get here, assumed this would
		// have been completely handled in `getUserSubscriptionStatus`
		return "trialExpired", nil
	}

	exp, err := time.Parse(time.RFC3339, s.Exp)
	if err != nil {
		return "", fmt.Errorf("unable to parse(%s): %s", s.Exp, err)
	}
	isActive := time.Now().Before(exp)

	name := strings.TrimSpace(s.Name)
	subType := name[strings.LastIndex(name, ".")+1:]

	if subType == "premiummonthly" {
		if isActive {
			return "monthly", nil
		} else {
			return "monthlyExpired", nil
		}
	} else if subType == "premiumannual" {
		if isActive {
			return "annual", nil
		} else {
			return "annualExpired", nil
		}
	}

	b, _ := json.Marshal(s)
	return "", fmt.Errorf("unable to handle subscription(%s)", string(b))
}

func getUserSubscriptionStatus(user *UserRow) (string, error) {
	if user.IsTrial != nil && *user.IsTrial && user.IsActive != nil {
		if !*user.IsActive {
			return "trialExpired", nil
		}

		if user.Name == nil {
			fmt.Printf("subscription name missing for user(%d)\n", user.ID)
			return "trial", nil
		}

		if *user.Name == "premiumannual" {
			return "trialAnnual", nil
		} else if *user.Name == "premiummonthly" {
			return "trialMonthly", nil
		} else {
			fmt.Printf("unknown subscription type: %s\n", user.Name)
			return "trial", nil
		}
	}

	if user.IsActiveCorporatePremium != nil {
		if *user.IsActiveCorporatePremium {
			return "freemium", nil
		} else {
			return "freemiumExpired", nil
		}
	}

	url := fmt.Sprintf(harborAPIBaseURL+"/admin/users/%d/subscriptions", user.ID)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", accessToken))

	resp, err := retryClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request for user(%d) failed: %s", user.ID, err)
	}

	if resp.StatusCode == 400 {
		return "invalid", nil
	} else if resp.StatusCode == 401 {
		authenticateAdminUser()
		req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", accessToken))
		resp, err = retryClient.Do(req)
		if err != nil {
			return "", fmt.Errorf("retry request for user(%d) failed: %s", user.ID, err)
		}
	} else if resp.StatusCode == 404 {
		return "notFound", nil
	}

	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := ioutil.ReadAll(resp.Body)
		tmplt := "%d get subscription response for user(%d): %s"
		return "", fmt.Errorf(tmplt, resp.StatusCode, user.ID, string(b))
	}

	var sub SubscriptionResp
	if err := json.NewDecoder(resp.Body).Decode(&sub); err != nil {
		return "", fmt.Errorf("unable to decode response for user(%d): %s", user.ID, err)
	}

	status, err := sub.GetStatus(user.ID)
	if err != nil {
		return "", fmt.Errorf("error getting status for user(%d): %s", user.ID, err)
	}

	return status, nil
}
