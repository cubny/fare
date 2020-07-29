package fare

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestNewSegment(t *testing.T) {
	p11 := Position{
		RideID:    1,
		Lat:       37.966660,
		Long:      23.728308,
		Timestamp: time.Unix(1405594957, 0),
	}
	p12 := Position{
		RideID:    1,
		Lat:       37.966627,
		Long:      23.728263,
		Timestamp: time.Unix(1405594966, 0),
	}
	p21 := Position{
		RideID:    2,
		Lat:       37.966627,
		Long:      23.728263,
		Timestamp: time.Unix(1405594966, 0),
	}
	tests := []struct {
		name     string
		p1, p2   Position
		maxSpeed float64
		check    func(s Segment, err error)
	}{
		{
			name:     "ok",
			p1:       p11,
			p2:       p12,
			maxSpeed: 100,
			check: func(s Segment, err error) {
				assert.Nil(t, err)
			},
		},
		{
			name:     "different rideId - error",
			p1:       p11,
			p2:       p21,
			maxSpeed: 100,
			check: func(s Segment, err error) {
				assert.NotNil(t, err)
			},
		},
		{
			name:     "exceeds max speed - error",
			p1:       p11,
			p2:       p12,
			maxSpeed: 1,
			check: func(s Segment, err error) {
				assert.NotNil(t, err)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.check(NewSegment(test.p1, test.p2, test.maxSpeed))
		})
	}
}

func TestSegment_Price(t *testing.T) {
	tests := []struct {
		name    string
		segment Segment
		check   func(p Price)
		fare    Price
	}{
		{
			name: "less than 10km/h",
			segment: Segment{
				speed:    5,
				duration: time.Hour,
			},
			fare: Price(fareIdlePerHour),
		},
		{
			name: "the minimum fare",
			segment: Segment{
				speed:      15,
				distance:   1,
				startedAt:  time.Unix(1405594957, 0),
				finishedAt: time.Unix(1405594965, 0),
			},
			fare: Price(fareMovingNormal),
		},
		{
			name: "midnight ride",
			segment: Segment{
				speed:      50,
				distance:   100,
				startedAt:  time.Unix(1593397864, 0),
				finishedAt: time.Unix(1593397964, 0),
			},
			fare: Price(130),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.fare, test.segment.Fare())
		})
	}
}

func BenchmarkNewSegment(b *testing.B) {
	p11 := Position{
		RideID:    1,
		Lat:       37.966660,
		Long:      23.728308,
		Timestamp: time.Unix(1405594957, 0),
	}
	p12 := Position{
		RideID:    1,
		Lat:       37.966627,
		Long:      23.728263,
		Timestamp: time.Unix(1405594966, 0),
	}

	for n := 0; n < b.N; n++ {
		_, _ = NewSegment(p11, p12, 100)
	}
}

func BenchmarkSegment_Fare(b *testing.B) {
	segment := Segment{
		speed:      50,
		distance:   100,
		startedAt:  time.Unix(1593397864, 0),
		finishedAt: time.Unix(1593397964, 0),
	}

	for n := 0; n < b.N; n++ {
		_ = segment.Fare()
	}
}
