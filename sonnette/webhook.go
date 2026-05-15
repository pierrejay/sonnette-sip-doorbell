package main

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"time"
)

// webhookClient has its own 5s timeout so a slow/dead consumer cannot
// pile up goroutines holding connections forever.
var webhookClient = &http.Client{Timeout: 5 * time.Second}

// PostEvent fires a JSON POST to the configured webhook URL in the
// background. Returns immediately; failures are logged, never bubbled
// up — the doorbell call itself is the primary user-facing signal,
// the webhook is best-effort notification.
//
// fields are merged into the JSON object on top of the standard
// "event" + "ts" envelope.
func PostEvent(cfg Config, eventType string, fields map[string]any) {
	if cfg.EventWebhookURL == "" {
		return
	}
	payload := map[string]any{
		"event": eventType,
		"ts":    time.Now().UTC().Format(time.RFC3339),
	}
	for k, v := range fields {
		payload[k] = v
	}
	go postWebhook(cfg.EventWebhookURL, eventType, payload)
}

func postWebhook(url, eventType string, payload map[string]any) {
	body, err := json.Marshal(payload)
	if err != nil {
		log.Printf("webhook: marshal %s: %v", eventType, err)
		return
	}
	resp, err := webhookClient.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		log.Printf("webhook: post %s: %v", eventType, err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		log.Printf("webhook: %s -> %d", eventType, resp.StatusCode)
	}
}

// StartWebhookHeartbeat launches a goroutine that POSTs a "heartbeat"
// event at the configured interval. No-op if the URL is empty or the
// interval is non-positive.
func StartWebhookHeartbeat(cfg Config, pjsua *PjsuaManager) {
	if cfg.EventWebhookURL == "" || cfg.EventWebhookHeartbeatSec <= 0 {
		return
	}
	interval := time.Duration(cfg.EventWebhookHeartbeatSec) * time.Second
	log.Printf("webhook: heartbeat every %s", interval)
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			PostEvent(cfg, "heartbeat", map[string]any{
				"uptime": time.Since(startTime).Truncate(time.Second).String(),
				"pjsua":  pjsua.IsRunning(),
				"wifi":   wifiState(),
			})
		}
	}()
}
