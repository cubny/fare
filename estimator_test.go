package fare

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEstimator_Run(t *testing.T) {
	data := `1,37.966660,23.728308,1405594957
1,37.966627,23.728263,1405594966
2,37.966660,23.728308,1405594957
2,37.966627,23.728263,1405594966`

	in := strings.NewReader(data)
	out := &bytes.Buffer{}

	options := &Config{
		MaxSpeed:    100,
		Concurrency: 1,
	}

	estimator, err := NewEstimator(in, out, options)
	assert.Nil(t, err)

	err = estimator.Run(context.TODO())
	assert.Nil(t, err)

	assert.Equal(t, "1,3.47\n2,3.47\n", out.String())
}
