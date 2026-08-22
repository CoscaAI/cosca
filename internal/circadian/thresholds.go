package circadian

import "time"

// Config holds the idle thresholds that drive the ORC cycle.
type Config struct {
	// IdleToResting is how long the engine may stay idle before the cycle
	// proposes the resting state. Default: 5 minutes.
	IdleToResting time.Duration

	// RestingToSleeping is how long the engine may stay idle before the
	// cycle proposes the ORC window (the sleeping state). Default: 15 minutes.
	RestingToSleeping time.Duration

	// AwakeTimeout is how long the engine stays fully awake after the last
	// activity before the cycle proposes the idle state. Default: 30 seconds.
	AwakeTimeout time.Duration
}

// DefaultConfig returns the standard ORC thresholds.
func DefaultConfig() Config {
	return Config{
		IdleToResting:     5 * time.Minute,
		RestingToSleeping: 15 * time.Minute,
		AwakeTimeout:      30 * time.Second,
	}
}
