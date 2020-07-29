package fare

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestNewPosition(t *testing.T) {
	tests := []struct {
		name     string
		tuple    []string
		hasError bool
	}{
		{
			name:     "ok",
			tuple:    []string{"1", "37.942437", "23.642862", "1405595819"},
			hasError: false,
		},
		{
			name:     "wrong rideId - error",
			tuple:    []string{"a", "37.942437", "23.642862", "1405595819"},
			hasError: true,
		},
		{
			name:     "wrong lat - error",
			tuple:    []string{"1", "a", "23.642862", "1405595819"},
			hasError: true,
		},
		{
			name:     "wrong long - error",
			tuple:    []string{"1", "37.942437", "a", "1405595819"},
			hasError: true,
		},
		{
			name:     "wrong time - error",
			tuple:    []string{"1", "37.942437", "23.642862", "a"},
			hasError: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewPosition(test.tuple[0], test.tuple[1], test.tuple[2], test.tuple[3])
			assert.Equal(t, test.hasError, err != nil)
		})
	}
}

func TestPosition_Distance(t *testing.T) {
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
	distance := p11.Distance(p12)
	assert.Equal(t, 0.005387608950290441, distance)
}

func BenchmarkNewPosition(b *testing.B) {
	tuple := []string{"1", "37.942437", "23.642862", "1405595819"}
	for n := 0; n < b.N; n++ {
		_, _ = NewPosition(tuple[0], tuple[1], tuple[2], tuple[3])
	}
}
