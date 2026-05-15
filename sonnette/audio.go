package main

import (
	"fmt"
	"log"
	"os/exec"
)

// SetupAudio configures ALSA mixer settings at startup.
func SetupAudio() {
	// PDM0 Gain Volume to 100% (numid=7 on card 0)
	out, err := exec.Command("amixer", "-c", "0", "cset", "numid=7", "100%").CombinedOutput()
	if err != nil {
		log.Printf("audio setup: amixer failed: %v (%s)", err, out)
		return
	}
	log.Println("audio: PDM gain set to 100%")
}

// PlayDingDong blocks until aplay finishes so the speaker (hw:1,0) is
// released before Twilio bridges the call back to pjsua. Without this,
// pjsua races aplay for the sound device and randomly fails with
// PJMEDIA_ENCSAMPLESPFRAME when the user picks up before dingdong ends.
func PlayDingDong(cfg Config) error {
	cmd := exec.Command("aplay", "-D", cfg.AplayDevice, cfg.DingdongWav)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("aplay: %w", err)
	}
	return nil
}
