package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lmsgprefix)
	log.SetPrefix("sonnette: ")

	cfg, err := LoadConfig()
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	log.Printf("config loaded (gpio_pin=%d, sip=%s@%s)",
		cfg.GPIOPin, cfg.SIPUsername, cfg.SIPDomain)

	// Setup ALSA: PDM mic gain to max
	SetupAudio()

	// Start pjsua supervisor
	pjsua := NewPjsuaManager(cfg)
	if err := pjsua.Start(); err != nil {
		log.Fatalf("pjsua start: %v", err)
	}

	// Health check endpoint (for monitoring)
	StartHealthServer(cfg, pjsua)

	// Optional outbound webhook (heartbeat + ring/call events)
	StartWebhookHeartbeat(cfg, pjsua)

	// Setup GPIO watcher
	gpio, err := NewGPIOWatcher(cfg.GPIOPin)
	if err != nil {
		log.Fatalf("gpio: %v", err)
	}
	defer gpio.Close()
	log.Printf("gpio%d ready (active low, falling edge)", cfg.GPIOPin)

	// Cooldown
	cd := NewCooldown(cfg.CooldownSec, cfg.MaxCallsPerHr)

	// Signal handler for clean shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		log.Println("shutting down")
		pjsua.Stop()
		gpio.Close()
		os.Exit(0)
	}()

	log.Println("ready — waiting for button press")

	for {
		if err := gpio.WaitForPress(); err != nil {
			log.Printf("gpio wait: %v", err)
			continue
		}

		log.Println("button pressed!")
		RecordButtonPress()

		if !cd.Allow() {
			log.Printf("cooldown active (%ds remaining)", cd.Remaining())
			PostEvent(cfg, "button_press", map[string]any{
				"outcome":            "cooldown",
				"cooldown_remaining": cd.Remaining(),
			})
			continue
		}

		if !pjsua.IsRunning() {
			log.Println("pjsua not running, skipping call")
			PostEvent(cfg, "button_press", map[string]any{"outcome": "pjsua_down"})
			continue
		}

		PostEvent(cfg, "button_press", map[string]any{"outcome": "calling"})

		// Play ding-dong synchronously: blocks until aplay releases
		// the speaker, so pjsua doesn't race for hw:1,0 if the user
		// picks up before the chime ends. See audio.go.
		if err := PlayDingDong(cfg); err != nil {
			log.Printf("dingdong: %v", err)
		}

		// Call via Twilio
		sid, err := TwilioCall(cfg)
		if err != nil {
			log.Printf("twilio call failed: %v", err)
			PostEvent(cfg, "call_failed", map[string]any{"error": err.Error()})
			continue
		}
		log.Printf("call initiated: %s", sid)
		RecordCallInitiated(sid)
		PostEvent(cfg, "call_initiated", map[string]any{"sid": sid})
	}
}
