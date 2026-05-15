package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"sync"
	"time"
)

type PjsuaManager struct {
	cfg     Config
	cmd     *exec.Cmd
	stdinW  *os.File // keep alive to prevent GC closing the pipe
	mu      sync.Mutex
	running bool
	stopCh  chan struct{}
}

func NewPjsuaManager(cfg Config) *PjsuaManager {
	return &PjsuaManager{
		cfg:    cfg,
		stopCh: make(chan struct{}),
	}
}

func (p *PjsuaManager) Start() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.running {
		return nil
	}

	go p.supervise()
	return nil
}

func (p *PjsuaManager) supervise() {
	for {
		select {
		case <-p.stopCh:
			return
		default:
		}

		log.Println("pjsua: starting")

		// Write SIP credentials to a temp file so they don't appear
		// in `ps` output (--password on the CLI is visible to all).
		// The file lives in /run (tmpfs) and is chmod 0600.
		const cfgFile = "/run/pjsua-account.cfg"
		accountCfg := fmt.Sprintf(
			"--registrar sip:%s;transport=tls\n--id sip:%s@%s\n--realm %s\n--username %s\n--password %s\n",
			p.cfg.SIPDomain,
			p.cfg.SIPUsername, p.cfg.SIPDomain,
			p.cfg.SIPRealm,
			p.cfg.SIPUsername,
			p.cfg.SIPPassword,
		)
		if err := os.WriteFile(cfgFile, []byte(accountCfg), 0600); err != nil {
			log.Printf("pjsua: cannot write %s: %v", cfgFile, err)
			log.Println("pjsua: REFUSING to start — credentials would leak in ps output")
			time.Sleep(10 * time.Second)
			continue
		}

		args := []string{
			"--config-file", cfgFile,
			"--stereo",
			"--clock-rate", "8000",
			"--snd-clock-rate", fmt.Sprintf("%d", p.cfg.SndClockRate),
			"--ptime", "20",
			"--quality", "10",
			"--dis-codec", ".*",
			"--add-codec", "pcmu",
			"--jb-max-size", "300",
			"--playback-lat", "200",
			"--no-vad",
			"--ec-tail", "0",
			"--auto-answer", "200",
			"--use-tls",       // SIP signaling over TLS (port 5061)
			"--no-udp",        // force TLS-only: no UDP fallback
			"--no-tcp",        // no plain TCP either
			"--use-srtp", "1", // prefer SRTP (encrypted audio)
			"--srtp-secure", "1", // require TLS for SRTP
			"--stun-srv", "global.stun.twilio.com",
			"--log-level", fmt.Sprintf("%d", p.cfg.PjsuaLogLevel),
			// Don't use pjsua's own --log-file: it was producing empty files
			// because it buffers without flushing. We capture stdout+stderr
			// to PjsuaLogFile directly below (see cmd.Stdout/Stderr).
		}

		// Run pjsua via stdbuf so its stdio is line-buffered instead of
		// block-buffered. When pjsua writes to a file (not a TTY), libc
		// defaults to 4KB block buffering and logs only land after the
		// buffer fills — we'd miss "registration success" for minutes.
		stdbufArgs := append([]string{"-oL", "-eL", p.cfg.PjsuaBinary}, args...)
		p.cmd = exec.Command("stdbuf", stdbufArgs...)
		// Write pjsua's stdout+stderr to a dedicated file instead of
		// inheriting sonnette's os.Stdout. When sonnette is supervised by
		// init with `| logger`, the inherited pipe can buffer/block pjsua
		// and break SIP timing (missed REGISTER, dropped INVITEs).
		pjsuaLog, err := os.OpenFile(p.cfg.PjsuaLogFile,
			os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
		if err != nil {
			log.Printf("pjsua: cannot open log file %s: %v — falling back to stdout",
				p.cfg.PjsuaLogFile, err)
			p.cmd.Stdout = os.Stdout
			p.cmd.Stderr = os.Stderr
		} else {
			p.cmd.Stdout = pjsuaLog
			p.cmd.Stderr = pjsuaLog
		}
		// Keep stdin open (pipe that never closes) so pjsua doesn't exit on EOF
		stdinR, stdinW, _ := os.Pipe()
		p.cmd.Stdin = stdinR
		p.stdinW = stdinW // prevent GC from closing the write end

		p.mu.Lock()
		p.running = true
		p.mu.Unlock()

		err = p.cmd.Run()

		p.mu.Lock()
		p.running = false
		p.mu.Unlock()

		if err != nil {
			log.Printf("pjsua: exited: %v", err)
		} else {
			log.Println("pjsua: exited cleanly")
		}

		select {
		case <-p.stopCh:
			return
		default:
			log.Println("pjsua: restarting in 3s")
			time.Sleep(3 * time.Second)
		}
	}
}

func (p *PjsuaManager) IsRunning() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.running
}

func (p *PjsuaManager) Stop() {
	close(p.stopCh)
	p.mu.Lock()
	cmd := p.cmd
	p.mu.Unlock()

	if cmd != nil && cmd.Process != nil {
		cmd.Process.Signal(os.Interrupt)
		done := make(chan struct{})
		go func() {
			cmd.Wait()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			cmd.Process.Kill()
		}
	}
}
