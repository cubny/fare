package fare

import (
	"strconv"
	"time"

	"github.com/cubny/fare/internal/haversine"
)

// Position holds the geo coordinates of a ride in specific time
type Position struct {
	RideID    int
	Lat, Long float64
	Timestamp time.Time
}

// NewPosition creates a Position out of a tuple of strings
func NewPosition(rawRideID, rawLat, rawLong, rawTimestamp string) (Position, error) {
	rideId, err := strconv.Atoi(rawRideID)
	if err != nil {
		return Position{}, err
	}

	lat, err := strconv.ParseFloat(rawLat, 6)
	if err != nil {
		return Position{}, err
	}

	long, err := strconv.ParseFloat(rawLong, 6)
	if err != nil {
		return Position{}, err
	}
	ti, err := strconv.ParseInt(rawTimestamp, 10, 64)
	if err != nil {
		return Position{}, err
	}
	timestamp := time.Unix(ti, 0)

	return Position{
		RideID:    rideId,
		Lat:       lat,
		Long:      long,
		Timestamp: timestamp,
	}, nil
}

// Distance returns the haversine distance of the given position
func (p Position) Distance(from Position) float64 {
	return haversine.Haversine(from.Long, from.Lat, p.Long, p.Lat)
}
