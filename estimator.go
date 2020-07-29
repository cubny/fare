package fare

import (
	"context"
	"encoding/csv"
	"errors"
	"io"
	"log"
	"strconv"

	"github.com/cubny/fare/internal/pipeline"
)

// estimator takes a reader stream of rides' positions and streams out the fare estimate
// of each ride into the writer stream
type estimator struct {
	reader io.Reader
	writer io.Writer
	conf   *Config
}

// NewEstimator creates a estimator struct
func NewEstimator(in io.Reader, out io.Writer, config *Config) (*estimator, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}

	return &estimator{
		reader: in,
		writer: out,
		conf:   config,
	}, nil
}

// Run runs the estimator pipeline
func (e *estimator) Run(ctx context.Context) error {
	in := csv.NewReader(e.reader)
	linec, errc1 := pipeline.Generate(ctx, e.streamFromCSV(in))
	ridec, errc2 := pipeline.Group(ctx, linec, e.groupByRideID)
	outc, errc3 := pipeline.WorkerPool(ctx, e.conf.Concurrency, ridec, e.estimateRide)
	if err := e.sinkCSV(ctx, outc); err != nil {
		return err
	}

	errm := pipeline.MergeErrors(ctx, errc1, errc2, errc3)
	for err := range errm {
		switch {
		case err == io.EOF:
		case err != nil:
			return err
		}
	}

	return nil
}

// groupByRideId is a pipeline.belongFunc that groups positions by rideId
func (e *estimator) groupByRideID(item interface{}, group []interface{}) (bool, error) {
	line, ok := item.(Line)
	if !ok {
		return false, errors.New("item of the wrong type passed")
	}
	first, ok := group[0].(Line)
	return line[0] == first[0], nil
}

// streamFromCSV returns pipeline.generatFunc that reads one line at a time froma csv.Reader
func (e *estimator) streamFromCSV(in *csv.Reader) func() (interface{}, error) {
	return func() (interface{}, error) {
		record, err := in.Read()
		return Line(record), err
	}
}

// sinkCSVRecord writes a rideFare record to csv.Writer
func (e *estimator) sinkCSVRecord(w *csv.Writer) func(interface{}) error {
	return func(val interface{}) error {
		rideFare, ok := val.(rideFare)
		if !ok {
			log.Printf("not of the type ride result")
			return nil
		}
		fareEstimate := strconv.FormatFloat(float64(rideFare.fare), 'f', 2, 64)
		rideId := strconv.Itoa(rideFare.rideId)
		record := Line{rideId, fareEstimate}
		err := w.Write(record)
		if err != nil {
			return err
		}
		return nil
	}
}

// sinkCSV writes all rideFare records to estimator writer in CSV format
func (e *estimator) sinkCSV(ctx context.Context, outc <-chan pipeline.Event) error {
	output := csv.NewWriter(e.writer)
	err := pipeline.Sink(ctx, outc, e.sinkCSVRecord(output))
	if err != nil {
		return err
	}

	output.Flush()
	if err := output.Error(); err != nil {
		return err
	}

	return nil
}

// estimateRide is a pipeline.workerFunc that runs the rideEstimator pipeline for each ride
func (e *estimator) estimateRide(ctx context.Context, items interface{}, outc chan<- pipeline.Event) error {
	var lines []Line
	for _, item := range items.([]interface{}) {
		lines = append(lines, item.(Line))
	}

	rideEstimator, err := newRide(lines, e.conf)
	if err != nil {
		return err
	}
	rideEstimator.run(ctx, outc)
	return nil
}
