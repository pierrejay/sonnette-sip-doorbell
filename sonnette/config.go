package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	// Twilio REST API
	TwilioAccountSID string `yaml:"twilio_account_sid"`
	TwilioAuthToken  string `yaml:"twilio_auth_token"`
	TwilioFromNumber string `yaml:"twilio_from_number"`
	TwilioToNumber   string `yaml:"twilio_to_number"`
	TwilioTwimlURL   string `yaml:"twilio_twiml_url"`

	// SIP / pjsua
	SIPUsername string `yaml:"sip_username"`
	SIPPassword string `yaml:"sip_password"`
	SIPDomain   string `yaml:"sip_domain"`
	SIPRealm    string `yaml:"sip_realm"`

	// GPIO (button, exposed via sysfs at /sys/class/gpio/gpio<GPIOPin>)
	GPIOPin int `yaml:"gpio_pin"`

	// Audio
	DingdongWav  string `yaml:"dingdong_wav"`
	AplayDevice  string `yaml:"aplay_device"`
	SndClockRate int    `yaml:"snd_clock_rate"`

	// Cooldown
	CooldownSec   int `yaml:"cooldown_sec"`
	MaxCallsPerHr int `yaml:"max_calls_per_hour"`

	// Health check
	HealthPort int `yaml:"health_port"`

	// Event webhook (push notifications to a remote HTTP endpoint).
	// Empty URL disables the feature. The URL may carry a secret token
	// path segment — it can also be supplied via EVENT_WEBHOOK_URL in
	// the credentials file. Heartbeat 0 disables the periodic event.
	EventWebhookURL          string `yaml:"event_webhook_url"`
	EventWebhookHeartbeatSec int    `yaml:"event_webhook_heartbeat_sec"`

	// Credentials file (shell KEY=value format, e.g. /etc/sonnette.conf)
	CredentialsFile string `yaml:"credentials_file"`

	// pjsua binary
	PjsuaBinary   string `yaml:"pjsua_binary"`
	PjsuaLogFile  string `yaml:"pjsua_log_file"`
	PjsuaLogLevel int    `yaml:"pjsua_log_level"`
}

func DefaultConfig() Config {
	return Config{
		SIPRealm:        "sip.twilio.com",
		GPIOPin:         0,
		DingdongWav:     "/root/dingdong.wav",
		AplayDevice:     "default",
		SndClockRate:    16000,
		CooldownSec:     30,
		MaxCallsPerHr:   10,
		HealthPort:      8080,
		CredentialsFile: "/etc/sonnette.conf",
		PjsuaBinary:     "/usr/bin/pjsua",
		PjsuaLogFile:    "/var/log/pjsua.log",
		PjsuaLogLevel:   3,
	}
}

func LoadConfig() (Config, error) {
	cfg := DefaultConfig()

	// 1. YAML file
	configPath := flag.String("config", "/etc/sonnette.yaml", "config file path")
	flag.Parse()

	data, err := os.ReadFile(*configPath)
	if err == nil {
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return cfg, fmt.Errorf("parse %s: %w", *configPath, err)
		}
	}

	// 2. Credentials file (shell KEY="value" format) overrides YAML
	if cfg.CredentialsFile != "" {
		creds, err := parseShellVars(cfg.CredentialsFile)
		if err != nil {
			return cfg, fmt.Errorf("credentials file %s: %w", cfg.CredentialsFile, err)
		}
		shellOverride(&cfg.TwilioAccountSID, creds, "TWILIO_ACCOUNT_SID")
		shellOverride(&cfg.TwilioAuthToken, creds, "TWILIO_AUTH_TOKEN")
		shellOverride(&cfg.TwilioFromNumber, creds, "TWILIO_FROM_NUMBER")
		shellOverride(&cfg.TwilioToNumber, creds, "TWILIO_TO_NUMBER")
		shellOverride(&cfg.TwilioTwimlURL, creds, "TWILIO_TWIML_URL")
		shellOverride(&cfg.SIPUsername, creds, "SIP_USERNAME")
		shellOverride(&cfg.SIPPassword, creds, "SIP_PASSWORD")
		shellOverride(&cfg.SIPDomain, creds, "SIP_DOMAIN")
		shellOverride(&cfg.SIPRealm, creds, "SIP_REALM")
		shellOverride(&cfg.EventWebhookURL, creds, "EVENT_WEBHOOK_URL")
	}

	// Validate required fields
	if cfg.TwilioAccountSID == "" || cfg.TwilioAuthToken == "" {
		return cfg, fmt.Errorf("twilio_account_sid and twilio_auth_token are required")
	}
	if cfg.SIPUsername == "" || cfg.SIPPassword == "" {
		return cfg, fmt.Errorf("sip_username and sip_password are required")
	}
	if cfg.SIPDomain == "" {
		return cfg, fmt.Errorf("sip_domain is required (e.g. yourdomain.sip.us1.twilio.com)")
	}
	if cfg.TwilioToNumber == "" {
		return cfg, fmt.Errorf("twilio_to_number is required")
	}

	return cfg, nil
}

func shellOverride(field *string, vars map[string]string, key string) {
	if v, ok := vars[key]; ok && v != "" {
		*field = v
	}
}

// parseShellVars reads a shell-style KEY=value file (the format used by
// sonnette.conf). Handles KEY=value, KEY="value", KEY='value', comments,
// and blank lines. Does not evaluate shell expansions.
func parseShellVars(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	vars := make(map[string]string)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Skip "export KEY" lines (but handle "export KEY=value")
		line = strings.TrimPrefix(line, "export ")
		line = strings.TrimSpace(line)

		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		// Strip surrounding quotes
		if len(v) >= 2 && ((v[0] == '"' && v[len(v)-1] == '"') || (v[0] == '\'' && v[len(v)-1] == '\'')) {
			v = v[1 : len(v)-1]
		}
		vars[k] = v
	}
	return vars, scanner.Err()
}
