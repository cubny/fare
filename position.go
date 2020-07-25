package fairestimation

import (
	"fmt"
	"github.com/cubny/fair-estimation/internal/haversine"
	"strconv"
	"time"
)

type Position struct {
	RideID    int
	Lat, Long float64
	Timestamp time.Time
}

func (p Position) String() string {
	return fmt.Sprintf("RideID:%d Lat:%f Long:%f Timestamp:%s", p.RideID, p.Lat, p.Long, p.Timestamp.Format(time.RFC3339Nano))
}

func (p Position) Distance(from Position) float64 {
	return haversine.Haversine(from.Long, from.Lat, p.Long, p.Lat)
}

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
