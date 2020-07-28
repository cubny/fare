package main

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"runtime/pprof"
	"strconv"
	"sync"

	fairestimation "github.com/cubny/fair-estimation"
)

type Line []string

func main() {
	cpuProfile, _ := os.Create("cpuprofile")
	memProfile, _ := os.Create("memprofile")
	pprof.StartCPUProfile(cpuProfile)

	in, err := os.Open(os.Args[1])
	if err != nil {
		log.Fatalf("open input file: %s", err)
	}

	concurrency, err := strconv.Atoi(os.Args[2])
	if err != nil {
		log.Fatalf("concurrency: %s", err)
	}

	out, err := os.Create(os.Args[3])
	if err != nil {
		log.Fatalf("open output in: %s", err)
	}

	defer func() {
		if err := in.Close(); err != nil {
			log.Printf("close input file: %s", err)
		}
		if err := out.Close(); err != nil {
			log.Printf("close output file: %s", err)
		}
	}()

	ctx, stop := context.WithCancel(context.Background())
	sigint := make(chan os.Signal, 1)
	signal.Notify(sigint, os.Interrupt)

	go func() {
		if err := EstimateAll(ctx, concurrency, in, out); err != nil {
			log.Printf("got error: %s\n", err)
		}
		sigint <- os.Interrupt
	}()
	<-sigint
	log.Printf("Caught ctrl-c...")
	stop()
	fmt.Println("writing profiles")
	pprof.StopCPUProfile()
	pprof.WriteHeapProfile(memProfile)
	fmt.Println("exit")

}

type RideResult struct {
	id    int
	price fairestimation.Price
	err   error
}

type workerFunc func(outc chan<- interface{})

func pool(concurrency int, consume workerFunc) <-chan interface{} {
	var wg sync.WaitGroup
	outc := make(chan interface{})

	wg.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		go func() {
			consume(outc)
			wg.Done()
		}()
	}

	go func() {
		wg.Wait()
		close(outc)
	}()
	return outc
}

func WriteFairs(ctx context.Context, w *csv.Writer, outc <-chan interface{}) error {
	for r := range outc {
		rideResult, ok := r.(RideResult)
		if !ok {
			log.Printf("not of the type ride result")
		}
		select {
		case <-ctx.Done():
			return errors.New("EstimateAll canceled")
		default:
			//if _, ok := fairs[f.id]; ok {
			//	log.Println("price already exists")
			//	continue
			//}
			if rideResult.err != nil {
				log.Println("price got error", rideResult.err)
				continue
			}
			//fairs[f.id] = f.price
			fairEstimate := strconv.FormatFloat(float64(rideResult.price), 'f', 2, 64)
			rideId := strconv.Itoa(rideResult.id)
			record := []string{rideId, fairEstimate}
			err := w.Write(record)
			if err != nil {
				return err
			}
		}
	}

	w.Flush()
	if err := w.Error(); err != nil {
		return err
	}

	return nil
}
func EstimateAll(ctx context.Context, concurrency int, r io.Reader, w io.WriteCloser) error {
	output := csv.NewWriter(w)

	linec, errc := GenLines(ctx, r)
	ridec, errc1 := AggregateRides(ctx, linec)
	fairc := pool(concurrency, func(outc chan<- interface{}) {
		EstimateRide(ctx, ridec, outc)
	})

	if err := WriteFairs(ctx, output, fairc); err != nil {
		return err
	}

	errm := mergeErrors(ctx, errc, errc1)
	for err := range errm {
		if err != nil {
			return err
		}
	}

	return nil
}

func GenLines(ctx context.Context, reader io.Reader) (<-chan Line, <-chan error) {
	linec := make(chan Line)
	errc := make(chan error)
	go func() {
		defer func() {
			close(linec)
			close(errc)
		}()
		in := csv.NewReader(reader)
		for {
			record, err := in.Read()
			if err == io.EOF {
				return
			}
			if err != nil {
				errc <- err
				return
			}
			select {
			case linec <- record:
			case <-ctx.Done():
				fmt.Println("reading lines canceled")
				return
			}
		}
	}()

	return linec, errc
}

func AggregateRides(ctx context.Context, linec <-chan Line) (<-chan interface{}, <-chan error) {
	rideLinec := make(chan interface{})
	errc := make(chan error)
	flush := func(ridesLines []Line) {
		if len(ridesLines) > 0 {
			select {
			case <-ctx.Done():
				return
			case rideLinec <- ridesLines:
			}
		}
	}

	go func() {
		var RideLines []Line
		lastRideID := ""
		defer func() {
			// flush the last aggregated lines
			flush(RideLines)
			close(rideLinec)
			close(errc)
		}()
		for line := range linec {
			select {
			case <-ctx.Done():
				errc <- errors.New("splitting rides canceled")
				return
			default:
			}
			if line[0] == lastRideID {
				RideLines = append(RideLines, line)
				lastRideID = line[0]
				continue
			}
			if len(RideLines) > 0 {
				rideLinec <- RideLines
			}
			RideLines, lastRideID = []Line{line}, line[0]
		}
	}()
	return rideLinec, errc
}

type SegmentPriceResult struct {
	RideID int
	price  fairestimation.Price
	err    error
}

type SegmentResult struct {
	segment fairestimation.Segment
	err     error
}

func GenSegments(ctx context.Context, positionResultc <-chan PositionResult) <-chan SegmentResult {
	segmentc := make(chan SegmentResult)
	go func() {
		defer close(segmentc)
		lastp := <-positionResultc
		for p := range positionResultc {
			segment, err := func(p PositionResult) (fairestimation.Segment, error) {
				defer func() {
					lastp = p
				}()
				if p.err != nil {
					return fairestimation.Segment{}, p.err
				}
				seg, err := fairestimation.NewSegment(lastp.position, p.position)
				if err != nil {
					return fairestimation.Segment{}, err
				}
				return seg, nil
			}(p)
			select {
			case segmentc <- SegmentResult{segment: segment, err: err}:
			case <-ctx.Done():
				return
			}
		}
	}()
	return segmentc
}

func EstimateSegment(ctx context.Context, segmentResultc <-chan SegmentResult) <-chan SegmentPriceResult {
	segmentFairc := make(chan SegmentPriceResult)
	go func() {
		defer close(segmentFairc)
		for segmentResult := range segmentResultc {
			if segmentResult.err != nil {
				// filter all erogenous segments
				continue
			}
			select {
			case segmentFairc <- SegmentPriceResult{RideID: segmentResult.segment.RideID(), price: segmentResult.segment.FairEst(), err: nil}:
			case <-ctx.Done():
				return
			}
		}
	}()
	return segmentFairc
}

func EstimateRide(ctx context.Context, ridec <-chan interface{}, fairc chan<- interface{}) {
	for lines := range ridec {
		ride, ok := lines.([]Line)
		if !ok {
			continue
		}
		//go func(ride []Line) {
		positionResultc := GenPositions(ctx, ride)
		segmentResultc := GenSegments(ctx, positionResultc)
		segmentFairc := EstimateSegment(ctx, segmentResultc)
		totalPrice := fairestimation.Price(0)
		var rideID int
		for sr := range segmentFairc {
			select {
			case <-ctx.Done():
				return
			default:
				if sr.err != nil {
					//fmt.Println(sr.err)
					continue
				}
				rideID = sr.RideID
				totalPrice += sr.price
			}
		}
		//if err := <-errc; err != nil {
		//	fairc <- RideResult{err: err}
		//	return
		//}

		select {
		case fairc <- RideResult{id: rideID, price: totalPrice}:
		case <-ctx.Done():
			return
		}
		//}(ride)
	}
}

type PositionResult struct {
	position fairestimation.Position
	err      error
}

func GenPositions(ctx context.Context, lines []Line) <-chan PositionResult {
	resultc := make(chan PositionResult)
	go func() {
		defer close(resultc)
		for _, r := range lines {
			pos, err := fairestimation.NewPosition(r[0], r[1], r[2], r[3])
			select {
			case resultc <- PositionResult{position: pos, err: err}:
			case <-ctx.Done():
				return
			}
		}
	}()
	return resultc
}

func mergeErrors(ctx context.Context, errs ...<-chan error) <-chan error {
	var wg sync.WaitGroup
	outc := make(chan error, len(errs))
	output := func(errc <-chan error) {
		defer wg.Done()
		for e := range errc {
			select {
			case outc <- e:
			case <-ctx.Done():
				return
			}
		}
	}

	wg.Add(len(errs))
	for _, errc := range errs {
		go output(errc)
	}

	go func() {
		wg.Wait()
		close(outc)
	}()

	return outc
}
