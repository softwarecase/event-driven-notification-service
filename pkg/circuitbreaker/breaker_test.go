package circuitbreaker

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCircuitBreaker_InitialState(t *testing.T) {
	cb := New(5, 3, 3, 30*time.Second)
	assert.Equal(t, StateClosed, cb.State())
	assert.True(t, cb.Allow())
}

func TestCircuitBreaker_OpensAfterThreshold(t *testing.T) {
	cb := New(3, 2, 2, 100*time.Millisecond)

	// Record failures up to threshold
	for i := 0; i < 3; i++ {
		assert.True(t, cb.Allow())
		cb.RecordFailure()
	}

	assert.Equal(t, StateOpen, cb.State())
	assert.False(t, cb.Allow())
}

func TestCircuitBreaker_TransitionsToHalfOpen(t *testing.T) {
	cb := New(2, 2, 2, 50*time.Millisecond)

	// Trip the breaker
	cb.RecordFailure()
	cb.RecordFailure()
	assert.Equal(t, StateOpen, cb.State())

	// Wait for reset timeout
	time.Sleep(60 * time.Millisecond)

	// Should transition to half-open
	assert.True(t, cb.Allow())
	assert.Equal(t, StateHalfOpen, cb.State())
}

func TestCircuitBreaker_ClosesAfterSuccessInHalfOpen(t *testing.T) {
	cb := New(2, 2, 3, 50*time.Millisecond)

	// Trip the breaker
	cb.RecordFailure()
	cb.RecordFailure()

	// Wait and transition to half-open
	time.Sleep(60 * time.Millisecond)
	assert.True(t, cb.Allow())
	assert.Equal(t, StateHalfOpen, cb.State())

	// Record successes
	cb.RecordSuccess()
	cb.RecordSuccess()
	assert.Equal(t, StateClosed, cb.State())
}

func TestCircuitBreaker_ReopensOnFailureInHalfOpen(t *testing.T) {
	cb := New(2, 2, 3, 50*time.Millisecond)

	// Trip the breaker
	cb.RecordFailure()
	cb.RecordFailure()

	// Wait and transition to half-open
	time.Sleep(60 * time.Millisecond)
	assert.True(t, cb.Allow())

	// Fail in half-open
	cb.RecordFailure()
	assert.Equal(t, StateOpen, cb.State())
}

func TestCircuitBreaker_HalfOpenMaxRequests(t *testing.T) {
	cb := New(2, 2, 2, 50*time.Millisecond)

	// Trip the breaker
	cb.RecordFailure()
	cb.RecordFailure()

	// Wait and transition to half-open
	time.Sleep(60 * time.Millisecond)
	assert.True(t, cb.Allow())  // transition call counts as 1st (halfOpenCount=1)
	assert.True(t, cb.Allow())  // halfOpenCount=2 (max)
	assert.False(t, cb.Allow()) // Rejected, at max
}

func TestCircuitBreaker_SuccessResetsFaiureCount(t *testing.T) {
	cb := New(3, 2, 2, 50*time.Millisecond)

	cb.RecordFailure()
	cb.RecordFailure()
	cb.RecordSuccess() // Should reset failure count

	failures, _ := cb.Counts()
	assert.Equal(t, 0, failures)

	// Need 3 more failures to trip
	cb.RecordFailure()
	cb.RecordFailure()
	assert.Equal(t, StateClosed, cb.State())
}
