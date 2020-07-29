package fare

import (
	"context"
	"github.com/cubny/fare/internal/pipeline"
	"github.com/stretchr/testify/assert"
	"log"
	"testing"
)

func TestRideEstimator_run(t *testing.T) {
	config := &Config{
		MaxSpeed:    100,
		Concurrency: 1,
	}
	lines := []Line{
		{"1", "37.966660", "23.728308", "1405594957"},
		{"1", "37.966627", "23.728263", "1405594966"},
		{"1", "37.966625", "23.728263", "1405594974"},
		{"1", "37.966613", "23.728375", "1405594984"},
		{"1", "37.966203", "23.728597", "1405594992"},
	}

	estimator, err := newRide(lines, config)
	assert.Nil(t, err)

	outc := make(chan pipeline.Event, 1)
	err = estimator.run(context.TODO(), outc)
	assert.Nil(t, err)

	o := <-outc
	log.Println("coming in", o)
	rideFare := o.(rideFare)
	assert.Equal(t, 1, rideFare.rideId)
	assert.Equal(t, Price(3.47), rideFare.fare)
	close(outc)

}
