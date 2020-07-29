package fare

import (
	"errors"
	"time"
)

// Segment is made of two consecutive positions of the same ride
type Segment struct {
	rideID     int
	speed      float64
	distance   float64
	duration   time.Duration
	startedAt  time.Time
	finishedAt time.Time
}

// NewSegment creates a Segment out of two Positions
// the maxSpeed is dismiss the outliers
func NewSegment(p1, p2 Position, maxSpeed float64) (Segment, error) {
	if p1.RideID != p2.RideID {
		return Segment{}, errors.New("ride is not the same")
	}
	startedAt := p1.Timestamp
	finishedAt := p2.Timestamp

	distance := p2.Distance(p1)

	duration := finishedAt.Sub(startedAt)
	speed := distance / duration.Hours()

	if speed < 0 || speed > maxSpeed {
		return Segment{}, errors.New("speed is out of range")
	}

	return Segment{
		rideID:     p1.RideID,
		speed:      speed,
		distance:   distance,
		duration:   duration,
		startedAt:  p1.Timestamp,
		finishedAt: p2.Timestamp,
	}, nil
}

// Fare estimates the fare of segment using business rules
// it assumes that segment is collected in short duration of time
// so it does not break the segment into two period of midnight hours and normal hours
func (s Segment) Fare() Price {
	switch {
	case s.speed <= 10:
		return Price(s.duration.Minutes() / 60 * fareIdlePerHour)
	case s.startedAt.Hour() >= 0 && s.finishedAt.Hour() <= 5:
		return Price(s.distance * fareMovingMidnight)
	default:
		return Price(s.distance * fareMovingNormal)
	}
}
