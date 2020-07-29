package fare

import (
	"context"
	"errors"
	"math"

	"github.com/cubny/fare/internal/pipeline"
)

var ErrLinesEmpty = errors.New("ride lines are empty")

// ride holds information of a ride which can run against its pipeline
// to estimate the ride's fare
type ride struct {
	rideId int
	lines  []Line
	conf   *Config
}

// rideFare is the result of ride pipeline
type rideFare struct {
	rideId int
	fare   Price
}

// newRide creates a ride
func newRide(lines []Line, conf *Config) (*ride, error) {
	if err := conf.Validate(); err != nil {
		return nil, err
	}

	return &ride{
		lines: lines,
		conf:  conf,
	}, nil
}

// run carries out the ride pipeline to estimate the ride fare
func (r *ride) run(ctx context.Context, outc chan<- pipeline.Event) error {
	positions, errc := pipeline.Generate(ctx, r.positions)
	segments, errc1 := pipeline.Reduce(ctx, positions, r.segments)
	total, err := r.fare(ctx, segments)
	if err != nil {
		return err
	}

	errm := pipeline.MergeErrors(ctx, errc, errc1)
	for err := range errm {
		switch {
		case err == ErrLinesEmpty:
		case err != nil:
			return err
		}
	}

	select {
	case <-ctx.Done():
	case outc <- total:
	}

	return nil
}

// positions is a pipeline.generateFunc which generates a stream of positions based on lines
func (r *ride) positions() (interface{}, error) {
	if len(r.lines) == 0 {
		return nil, ErrLinesEmpty
	}

	line := r.unshiftLines()
	position, err := NewPosition(line[0], line[1], line[2], line[3])
	if err != nil {
		return nil, nil
	}

	return position, nil
}

// segments is a pipeline.reduceFunc which reduces two consecutive positions into a segment
func (r *ride) segments(item1 interface{}, last interface{}) (interface{}, error) {
	p1 := item1.(Position)
	p2 := last.(Position)
	seg, err := NewSegment(p1, p2, r.conf.MaxSpeed)
	if err != nil {
		// erroneous segment will be skipped
		return nil, nil
	}
	return seg, nil
}

// fare calculates the total sum of the ride fare estimation
// fare is the sink of the ride pipeline
func (r *ride) fare(ctx context.Context, segments <-chan pipeline.Event) (rideFare, error) {
	totalFare := Price(fareFlag)
	rideId := 0
	err := pipeline.Sink(ctx, segments, func(val interface{}) error {
		item := val.(Segment)
		totalFare += item.Fare()
		rideId = item.rideID
		return nil
	})

	return rideFare{
		rideId: rideId,
		fare:   Price(math.Max(float64(totalFare), fareMinimum)),
	}, err
}

// unshiftLines unshifts a member from ride's lines
func (r *ride) unshiftLines() Line {
	line, lines := r.lines[0], r.lines[1:]
	r.lines = lines
	return line
}
