package pipeline

import (
	"context"
	"io"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerate(t *testing.T) {
	tests := []struct {
		name      string
		generator func() generateFunc
		check     func(items <-chan Event, errc <-chan error)
	}{
		{
			name: "generates 10 numbers",
			generator: func() generateFunc {
				i := 0
				return func() (interface{}, error) {
					i++
					if i <= 10 {
						return rand.Int(), nil
					}
					return 0, assert.AnError
				}
			},
			check: func(items <-chan Event, errc <-chan error) {
				count := 0
				for _ = range items {
					count++
				}
				assert.Equal(t, 10, count)
				assert.Equal(t, assert.AnError, <-errc)
			},
		},
		{
			name: "skips nil",
			generator: func() generateFunc {
				i := 0
				return func() (interface{}, error) {
					i++
					if i <= 10 {
						return nil, nil
					}
					return 0, assert.AnError
				}
			},
			check: func(items <-chan Event, errc <-chan error) {
				count := 0
				for _ = range items {
					count++
				}
				assert.Equal(t, 0, count)
				assert.Equal(t, assert.AnError, <-errc)
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.check(Generate(context.TODO(), test.generator()))
		})
	}
}

func TestGroup(t *testing.T) {
	tests := []struct {
		name   string
		inc    <-chan Event
		belong func() belongFunc
		check  func(items <-chan Event, errc <-chan error)
	}{
		{
			name: "group by number",
			inc:  generateInt(t, []int{0, 0, 0, 1, 1, 1}),
			belong: func() belongFunc {
				return func(item interface{}, group []interface{}) (bool, error) {
					citem := item.(int)
					first := group[0].(int)
					return citem == first, nil
				}
			},
			check: func(items <-chan Event, errc <-chan error) {
				count := 0
				for _ = range items {
					count++
				}
				assert.Equal(t, 2, count)
				assert.Nil(t, <-errc)
			},
		},
		{
			name: "belong returns error - drain happens",
			inc:  generateInt(t, []int{0, 0, 0, 1, 1, 1}),
			belong: func() belongFunc {
				i := 0
				return func(item interface{}, group []interface{}) (bool, error) {
					citem := item.(int)
					first := group[0].(int)
					if i == 4 {
						return false, assert.AnError
					}
					i++
					return citem == first, nil
				}
			},
			check: func(items <-chan Event, errc <-chan error) {
				var lastGroup []interface{}
				for item := range items {
					lastGroup = item.([]interface{})
				}
				assert.Equal(t, 2, len(lastGroup))
				assert.Equal(t, assert.AnError, <-errc)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.check(Group(context.TODO(), test.inc, test.belong()))
		})
	}
}

func TestSink(t *testing.T) {
	tests := []struct {
		name   string
		inc    <-chan Event
		sinker eachFunc
		check  func(err error)
	}{
		{
			name: "runs sink on all items",
			inc:  generateInt(t, []int{1, 2, 3, 4, 5}),
			sinker: func(val interface{}) error {
				return nil
			},
			check: func(err error) {
				assert.Nil(t, err)
			},
		},
		{
			name: "sinker interrupts the sink",
			inc:  generateInt(t, []int{1, 2, 3, 4, 5}),
			sinker: func(val interface{}) error {
				if val.(int) > 3 {
					return assert.AnError
				}
				return nil
			},
			check: func(err error) {
				assert.Equal(t, assert.AnError, err)
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.check(Sink(context.TODO(), test.inc, test.sinker))
		})
	}
}

func TestReduce(t *testing.T) {
	tests := []struct {
		name    string
		inc     <-chan Event
		reducer reduceFunc
		check   func(outc <-chan Event, errc <-chan error)
	}{
		{
			name: "adds last two items",
			inc:  generateInt(t, []int{1, 1, 2, 2}),
			reducer: func(i interface{}, j interface{}) (interface{}, error) {
				return i.(int) + j.(int), nil
			},
			check: func(outc <-chan Event, errc <-chan error) {
				total := 0
				for item := range outc {
					total += item.(int)
				}
				// (1+1) + (1+2) + (2+2) = 9
				assert.Equal(t, 9, total)
				assert.Nil(t, <-errc)
			},
		},
		{
			name: "interrupt the reduce by an error",
			inc:  generateInt(t, []int{1, 1, 2, 2}),
			reducer: func(i interface{}, j interface{}) (interface{}, error) {
				if i.(int) == 2 {
					return nil, assert.AnError
				}
				return i.(int) + j.(int), nil
			},
			check: func(outc <-chan Event, errc <-chan error) {
				total := 0
				for item := range outc {
					total += item.(int)
				}
				// (1+1) + (1+2) + (2...error) = 5
				assert.Equal(t, 5, total)
				assert.Equal(t, assert.AnError, <-errc)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.check(Reduce(context.TODO(), test.inc, test.reducer))
		})
	}
}

func TestWorkerPool(t *testing.T) {
	tests := []struct {
		name        string
		concurrency int
		inc         <-chan Event
		worker      workerFunc
		check       func(outc <-chan Event, errc <-chan error)
	}{
		{
			name:        "2 adders",
			concurrency: 2,
			inc:         generateInt(t, []int{1, 2, 3, 4, 5}),
			worker: func(ctx context.Context, item interface{}, outc chan<- Event) error {
				outc <- item.(int) * 10
				return nil
			},
			check: func(outc <-chan Event, errc <-chan error) {
				total := 0
				for item := range outc {
					total += item.(int)
				}
				assert.Equal(t, 150, total)
				assert.Nil(t, <-errc)
			},
		},
		{
			name:        "2 adders, just add the first three items",
			concurrency: 2,
			inc:         generateInt(t, []int{1, 2, 3, 4, 5}),
			worker: func(ctx context.Context, item interface{}, outc chan<- Event) error {
				if item.(int) > 3 {
					return assert.AnError
				}
				outc <- item.(int) * 10
				return nil
			},
			check: func(outc <-chan Event, errc <-chan error) {
				total := 0
				for item := range outc {
					total += item.(int)
				}
				assert.Greater(t, total, 0)
				assert.NotEqual(t, 150, total)
				assert.Equal(t, assert.AnError, <-errc)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.check(WorkerPool(context.TODO(), test.concurrency, test.inc, test.worker))
		})
	}
}

func TestMergeErrors(t *testing.T) {
	tests := []struct {
		name  string
		errs  []<-chan error
		check func(err <-chan error)
	}{
		{
			name: "3 error channel, one returns io.EOF",
			errs: func() []<-chan error {
				errc1 := make(chan error, 1)
				errc2 := make(chan error, 1)
				errc3 := make(chan error, 1)
				errc2 <- io.EOF
				return []<-chan error{errc1, errc2, errc3}
			}(),
			check: func(errc <-chan error) {
				assert.Equal(t, io.EOF, <-errc)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.check(MergeErrors(context.TODO(), test.errs...))
		})
	}
}

func generateInt(t *testing.T, items []int) <-chan Event {
	t.Helper()
	i := 0
	outc, _ := Generate(context.TODO(), func() (interface{}, error) {
		if i >= len(items) {
			return nil, assert.AnError
		}
		ret := items[i]
		i++
		return ret, nil
	})
	return outc
}
