package engine

import (
	"fmt"
	"sync"
	"time"
)

// Spinner represents a simple CLI text spinner.
type Spinner struct {
	mu       sync.Mutex
	active   bool
	stopChan chan struct{}
	message  string
}

// NewSpinner creates a new Spinner instance.
func NewSpinner(message string) *Spinner {
	return &Spinner{
		message: message,
	}
}

// Start starts the spinner animation in a background goroutine.
func (s *Spinner) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active {
		return
	}
	s.active = true
	s.stopChan = make(chan struct{})

	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	go func() {
		ticker := time.NewTicker(80 * time.Millisecond)
		defer ticker.Stop()
		i := 0
		for {
			select {
			case <-s.stopChan:
				return
			case <-ticker.C:
				s.mu.Lock()
				msg := s.message
				s.mu.Unlock()
				// Print spinner in magenta, message in regular text, clear remaining line
				fmt.Printf("\r\x1b[1;35m%s\x1b[0m %s\x1b[K", frames[i%len(frames)], msg)
				i++
			}
		}
	}()
}

// Stop stops the spinner animation and clears the spinner line.
func (s *Spinner) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.active {
		return
	}
	s.active = false
	close(s.stopChan)
	// Clear the current line
	fmt.Print("\r\x1b[K")
}

// SetMessage updates the spinner message dynamically.
func (s *Spinner) SetMessage(msg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.message = msg
}
