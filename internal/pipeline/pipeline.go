package pipeline

import (
	"context"
	"errors"
	"sync"
)

// Event is used as the type for input and output channels
type Event interface{}

type (
	// eachFunc is called for each event of the input channel
	eachFunc func(val interface{}) error
	// generateFunc is used in Generate to produce values for the output channel
	generateFunc func() (interface{}, error)
	// belongFunc checks if an item belongs to a group
	belongFunc func(item interface{}, group []interface{}) (bool, error)
	// workerFunc consumes an item of the input channel
	// and publishes the result to the output channel
	workerFunc func(ctx context.Context, item interface{}, outc chan<- Event) error
	// reduceFunc is called on two subsequent events of the input stream
	// and reduce them to one item to be published to the output channel
	reduceFunc func(i interface{}, j interface{}) (interface{}, error)
)

// Generate converts output of a generateFunc to channel of Event
// the only way to close the output channel is to return an error from the generateFunc
// if generateFunc returns nil as the value, Generate won't put it to the channel
func Generate(ctx context.Context, fn generateFunc) (<-chan Event, <-chan error) {
	outc := make(chan Event)
	errc := make(chan error, 1)
	go func() {
		defer func() {
			close(outc)
			close(errc)
		}()
		for {
			select {
			case <-ctx.Done():
				errc <- errors.New("generate canceled")
				return
			default:
			}
			res, err := fn()
			switch {
			case err != nil:
				errc <- err
				return
			case res != nil: // only non nil res is put to out channel
				outc <- res
			}
		}
	}()

	return outc, errc
}

// Group is a transformer that groups events by checking against a belongFunc
func Group(ctx context.Context, inc <-chan Event, belong belongFunc) (<-chan Event, <-chan error) {
	outc := make(chan Event)
	errc := make(chan error, 1)
	drain := func(group []interface{}) {
		if len(group) > 0 {
			select {
			case <-ctx.Done():
				return
			case outc <- group:
			}
		}
	}

	go func() {
		var group []interface{}
		defer func() {
			// drain the last group
			drain(group)
			close(outc)
			close(errc)
		}()
		for item := range inc {
			select {
			case <-ctx.Done():
				errc <- errors.New("splitting rides canceled")
				return
			default:
			}
			// if the group is empty it means this is the item received
			if len(group) == 0 {
				group = append(group, item)
				continue
			}
			ok, err := belong(item, group)
			if err != nil {
				errc <- err
				return
			}
			if ok {
				group = append(group, item)
				continue
			}
			// since the item did not belong to the group
			// group is put to the out channel
			if len(group) > 0 {
				outc <- group
			}
			// re-initialise the group by the current item
			group = []interface{}{item}

		}
	}()
	return outc, errc
}

// Sink is a sinker which runs an eachFunc on each event
// it is the final stage of the pipeline as it does not produce any channel
func Sink(ctx context.Context, ch <-chan Event, fn eachFunc) error {
	for r := range ch {
		select {
		case <-ctx.Done():
			return errors.New("sink canceled")
		default:
			if err := fn(r); err != nil {
				return err
			}
		}
	}
	return nil
}

// Reduce is a transformer that passes two subsequent events to a reduceFunc
// and put the result to the output channel when the result is not nil
func Reduce(ctx context.Context, inc <-chan Event, reduce reduceFunc) (<-chan Event, <-chan error) {
	outc := make(chan Event)
	errc := make(chan error, 1)
	go func() {
		defer func() {
			close(outc)
			close(errc)
		}()
		var last interface{}
		for item := range inc {
			// we need at least two items to pass to reduce
			if last == nil {
				last = item
				continue
			}
			result, err := reduce(last, item)

			switch {
			case err != nil:
				errc <- err
				return
			case result == nil: // only put non nil values to output channel
				continue
			default:
				last = item
			}

			select {
			case outc <- result:
			case <-ctx.Done():
				return
			}
		}
	}()
	return outc, errc
}

// WorkerPool fans out the input channel to N worker which all publish on the output channel
// if a worker returns an error during the consumption, the pool continues skips the current event
// and spawns the worker again for the next item
func WorkerPool(ctx context.Context, concurrency int, inc <-chan Event, worker workerFunc) (<-chan Event, <-chan error) {
	var wg sync.WaitGroup
	outc := make(chan Event)
	errc := make(chan error, concurrency)

	wg.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		go func() {
			defer wg.Done()
			for item := range inc {
				err := worker(ctx, item, outc)
				if err != nil {
					errc <- err
					// worker could not work out the current item
					// skip the current item but keep the worker
					continue
				}
			}
		}()
	}

	go func() {
		wg.Wait()
		close(outc)
		close(errc)
	}()

	return outc, errc
}

// MergeErrors is a transformer which merges all input error channels into one output channel
func MergeErrors(ctx context.Context, errs ...<-chan error) <-chan error {
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
