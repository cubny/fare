package fare

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name     string
		config   *Config
		hasError bool
	}{
		{
			name: "ok",
			config: &Config{
				MaxSpeed:    100,
				Concurrency: 2,
			},
			hasError: false,
		},
		{
			name: "speed is zero - error",
			config: &Config{
				MaxSpeed:    0,
				Concurrency: 2,
			},
			hasError: true,
		},
		{
			name: "concurrency is zero - error",
			config: &Config{
				MaxSpeed:    100,
				Concurrency: 0,
			},
			hasError: true,
		},
	}

	for _, test := range tests {
		err := test.config.Validate()
		assert.Equal(t, test.hasError, err != nil)
	}
}
