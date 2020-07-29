/*
	Package fare provides the required functionalities to estimate the fare of rides by accepting
	a list of tuples of the form (id_ride, lat, lng, timestamp) as input and performs a pipeline
	including the required business rules to estimate the fare for each ride and then outputs the
	result as (id_ride, fare_estimate)
*/
package fare

import "errors"

// Price is a type for price value
type Price float32

// Line is a slice of strings
type Line []string

const (
	// fare amounts based on business rules

	fareIdlePerHour    = 11.9
	fareMovingMidnight = 1.30
	fareMovingNormal   = 0.74
	fareFlag           = 1.30
	fareMinimum        = 3.47
)

type Config struct {
	MaxSpeed    float64
	Concurrency int
}

func (c Config) Validate() error {
	switch {
	case c.MaxSpeed == 0:
		return errors.New("MaxSpeed should be greater than 0")
	case c.Concurrency == 0:
		return errors.New("concurrency should be greater than 0")
	}

	return nil
}
