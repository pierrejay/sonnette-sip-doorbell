package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"strings"
	"time"
)

var startTime = time.Now()

type HealthResponse struct {
	Status            string `json:"status"`
	Uptime            string `json:"uptime"`
	Pjsua             bool   `json:"pjsua"`
	Wifi              string `json:"wifi"`
	LastButtonPress   string `json:"last_button_press,omitempty"`
	LastCallInitiated string `json:"last_call_initiated,omitempty"`
	LastCallSID       string `json:"last_call_sid,omitempty"`
}

func StartHealthServer(cfg Config, pjsua *PjsuaManager) {
	if cfg.HealthPort == 0 {
		return
	}

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		resp := HealthResponse{
			Status: "ok",
			Uptime: time.Since(startTime).Truncate(time.Second).String(),
			Pjsua:  pjsua.IsRunning(),
			Wifi:   wifiState(),
		}
		if rs := LoadRingState(); rs != nil {
			if !rs.LastButtonPress.IsZero() {
				resp.LastButtonPress = rs.LastButtonPress.UTC().Format(time.RFC3339)
			}
			if !rs.LastCallInitiated.IsZero() {
				resp.LastCallInitiated = rs.LastCallInitiated.UTC().Format(time.RFC3339)
				resp.LastCallSID = rs.LastCallSID
			}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	addr := fmt.Sprintf(":%d", cfg.HealthPort)
	log.Printf("health: listening on %s", addr)
	go func() {
		if err := http.ListenAndServe(addr, nil); err != nil {
			log.Printf("health: %v", err)
		}
	}()
}

func wifiState() string {
	out, err := exec.Command("wpa_cli", "-i", "wlan0", "status").Output()
	if err != nil {
		return "unknown"
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "wpa_state=") {
			return strings.TrimPrefix(line, "wpa_state=")
		}
	}
	return "unknown"
}
