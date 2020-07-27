package fairestimation

import (
	"errors"
	"time"
)

type Price float32

type Segment struct {
	p1, p2   Position
	speed    float64
	distance float64
	duration time.Duration
}

func NewSegment(p1, p2 Position) (Segment, error) {
	if p1.RideID != p2.RideID {
		return Segment{}, errors.New("ride is not the same")
	}
	distance := p2.Distance(p1)

	duration := p2.Timestamp.Sub(p1.Timestamp)
	speed := distance / duration.Hours()
	if speed > 100 {
		return Segment{}, errors.New("outlier")
	}

	return Segment{
		p1:       p1,
		p2:       p2,
		speed:    speed,
		distance: distance,
		duration: duration,
	}, nil
}

func (s Segment) RideID() int {
	return s.p1.RideID
}

func (s Segment) FairEst() Price {
	switch {
	case s.speed <= 10:
		return Price(s.duration.Minutes() / 60 * 11.9)
	case s.p1.Timestamp.Hour() >= 0 && s.p1.Timestamp.Hour() <= 5:
		return Price(s.distance * 0.74)
	default:
		return Price(s.distance * 1.30)
	}
}
