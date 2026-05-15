package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// twilioClient has an explicit timeout so a flaky/lost internet connection
// can't block the main loop for minutes (Go's DefaultClient has no timeout).
var twilioClient = &http.Client{Timeout: 10 * time.Second}

func TwilioCall(cfg Config) (string, error) {
	apiURL := fmt.Sprintf(
		"https://api.twilio.com/2010-04-01/Accounts/%s/Calls.json",
		cfg.TwilioAccountSID,
	)

	data := url.Values{
		"From": {cfg.TwilioFromNumber},
		"To":   {cfg.TwilioToNumber},
		"Url":  {cfg.TwilioTwimlURL},
	}

	req, err := http.NewRequest("POST", apiURL, strings.NewReader(data.Encode()))
	if err != nil {
		return "", fmt.Errorf("twilio request: %w", err)
	}
	req.SetBasicAuth(cfg.TwilioAccountSID, cfg.TwilioAuthToken)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := twilioClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("twilio post: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("twilio %d: %s", resp.StatusCode, body)
	}

	var result struct {
		SID string `json:"sid"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("twilio parse: %w", err)
	}

	return result.SID, nil
}
