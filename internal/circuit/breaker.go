package circuit

import (
	"errors"
	"sync"
	"time"
)

// State represents the circuit breaker state
type State int

const (
	Closed   State = iota // Normal operation
	Open                  // Failing, reject calls
	HalfOpen              // Testing if service recovered
)

var ErrCircuitOpen = errors.New("circuit breaker is open")

type Breaker struct {
	mu             sync.Mutex
	state          State
	failureCount   int
	successCount   int
	maxFailures    int
	resetTimeout   time.Duration
	halfOpenMax    int
	lastFailureAt  time.Time
}

func NewBreaker(maxFailures int, resetTimeout time.Duration) *Breaker {
	return &Breaker{
		state:        Closed,
		maxFailures:  maxFailures,
		resetTimeout: resetTimeout,
		halfOpenMax:  2,
	}
}

func (b *Breaker) Execute(fn func() error) error {
	b.mu.Lock()

	switch b.state {
	case Open:
		if time.Since(b.lastFailureAt) > b.resetTimeout {
			b.state = HalfOpen
			b.successCount = 0
			b.mu.Unlock()
		} else {
			b.mu.Unlock()
			return ErrCircuitOpen
		}
	case HalfOpen:
		b.mu.Unlock()
	default:
		b.mu.Unlock()
	}

	err := fn()

	b.mu.Lock()
	defer b.mu.Unlock()

	if err != nil {
		b.failureCount++
		b.lastFailureAt = time.Now()
		if b.failureCount >= b.maxFailures {
			b.state = Open
		}
		return err
	}

	if b.state == HalfOpen {
		b.successCount++
		if b.successCount >= b.halfOpenMax {
			b.state = Closed
			b.failureCount = 0
		}
	} else {
		b.failureCount = 0
	}

	return nil
}

func (b *Breaker) State() State {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.state
}

func (b *Breaker) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.state = Closed
	b.failureCount = 0
	b.successCount = 0
}
