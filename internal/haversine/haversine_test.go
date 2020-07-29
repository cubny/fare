package haversine

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestHaversine(t *testing.T) {
	tests := []struct {
		name     string
		latlangs []float64
		distance float64
	}{
		{
			name:     "ok",
			latlangs: []float64{23.730235, 37.967349, 23.730235, 37.967348},
			distance: 0.00011119492636381855,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			distance := Haversine(test.latlangs[0], test.latlangs[1], test.latlangs[2], test.latlangs[3])
			assert.Equal(t, test.distance, distance)
		})
	}
}

func BenchmarkHaversine(b *testing.B) {
	// run the Haversine function b.N times
	latlangs := []float64{23.730235, 37.967349, 23.730235, 37.967348}
	for n := 0; n < b.N; n++ {
		Haversine(latlangs[0], latlangs[1], latlangs[2], latlangs[3])
	}
}
