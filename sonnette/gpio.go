package main

import (
	"fmt"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

const debounceMs = 300

// GPIO v2 ioctl constants (kernel >= 5.10)
const (
	gpioV2GetLineIoctl       = 0xC250B407
	gpioV2LineEventIoctl     = 0xC010B40E
	gpioV2LineGetValuesIoctl = 0xC010B40E

	gpioV2LineFlagInput      = 0x04
	gpioV2LineFlagActiveLow  = 0x02
	gpioV2LineFlagBiasPullUp = 0x100
	gpioV2LineFlagEdgeFalling = 0x04
	gpioV2LineFlagEventFallingEdge = 0x04
)

// Simplified: use sysfs for the PoC, ioctl v2 for later.
// sysfs is reliable and doesn't require struct alignment games.

type GPIOWatcher struct {
	pin       int
	valueFd   int
	lastPress int64 // unix ms, for debounce
}

func NewGPIOWatcher(pin int) (*GPIOWatcher, error) {
	g := &GPIOWatcher{pin: pin}
	if err := g.setup(); err != nil {
		return nil, err
	}
	return g, nil
}

func (g *GPIOWatcher) setup() error {
	// Export
	writeFile("/sys/class/gpio/export", fmt.Sprintf("%d", g.pin))

	gpioPath := fmt.Sprintf("/sys/class/gpio/gpio%d", g.pin)

	// Direction = input
	if err := writeFile(gpioPath+"/direction", "in"); err != nil {
		return fmt.Errorf("gpio direction: %w", err)
	}

	// Active low
	if err := writeFile(gpioPath+"/active_low", "1"); err != nil {
		return fmt.Errorf("gpio active_low: %w", err)
	}

	// Edge = falling
	if err := writeFile(gpioPath+"/edge", "falling"); err != nil {
		return fmt.Errorf("gpio edge: %w", err)
	}

	// Open value fd for poll()
	fd, err := syscall.Open(gpioPath+"/value", syscall.O_RDONLY, 0)
	if err != nil {
		return fmt.Errorf("gpio open value: %w", err)
	}
	g.valueFd = fd

	// Initial read to clear any pending event
	buf := make([]byte, 16)
	syscall.Read(fd, buf)

	return nil
}

// WaitForPress blocks until the button is pressed (falling edge).
func (g *GPIOWatcher) WaitForPress() error {
	pollFd := []unix.PollFd{{
		Fd:     int32(g.valueFd),
		Events: unix.POLLPRI | unix.POLLERR,
	}}

	for {
		_, err := unix.Poll(pollFd, -1)
		if err != nil {
			if err == syscall.EINTR {
				continue
			}
			return fmt.Errorf("gpio poll: %w", err)
		}

		// Read and discard to clear the event
		syscall.Seek(g.valueFd, 0, 0)
		buf := make([]byte, 16)
		syscall.Read(g.valueFd, buf)

		// Debounce: ignore events within 300ms of last press
		now := time.Now().UnixMilli()
		if now-g.lastPress < debounceMs {
			continue
		}
		g.lastPress = now
		return nil
	}
}

func (g *GPIOWatcher) Close() {
	syscall.Close(g.valueFd)
	writeFile("/sys/class/gpio/unexport", fmt.Sprintf("%d", g.pin))
}

func writeFile(path, value string) error {
	fd, err := syscall.Open(path, syscall.O_WRONLY, 0)
	if err != nil {
		return err
	}
	defer syscall.Close(fd)
	_, err = syscall.Write(fd, []byte(value))
	return err
}
