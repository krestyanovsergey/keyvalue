package keyvalue

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func validConfig() Config {
	return Config{
		InitialBucketCount:      16,
		CollectorInterval:       time.Second,
		CollectorMaxIterations:  3,
		CollectorBatchRatio:     0.25,
		CollectorThresholdRatio: 0.5,
	}
}

func TestConfig_Validate_Valid(t *testing.T) {
	cfg := validConfig()

	assert.NoError(t, cfg.Validate())
}

func TestConfig_Validate_Boundaries(t *testing.T) {
	tests := []struct {
		name   string
		modify func(*Config)
	}{
		{"минимальное число корзин", func(c *Config) { c.InitialBucketCount = 1 }},
		{"минимальный интервал", func(c *Config) { c.CollectorInterval = time.Nanosecond }},
		{"минимум итераций", func(c *Config) { c.CollectorMaxIterations = 1 }},
		{"batch ratio равен 1", func(c *Config) { c.CollectorBatchRatio = 1 }},
		{"batch ratio чуть больше 0", func(c *Config) { c.CollectorBatchRatio = 0.0001 }},
		{"threshold ratio равен 1", func(c *Config) { c.CollectorThresholdRatio = 1 }},
		{"threshold ratio чуть больше 0", func(c *Config) { c.CollectorThresholdRatio = 0.0001 }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			tt.modify(&cfg)

			assert.NoError(t, cfg.Validate())
		})
	}
}

func TestConfig_Validate_Invalid(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(*Config)
		wantErr string
	}{
		{"корзин ноль", func(c *Config) { c.InitialBucketCount = 0 }, "InitialBucketCount"},
		{"корзин отрицательное число", func(c *Config) { c.InitialBucketCount = -1 }, "InitialBucketCount"},

		{"интервал ноль", func(c *Config) { c.CollectorInterval = 0 }, "CollectorInterval"},
		{"интервал отрицательный", func(c *Config) { c.CollectorInterval = -time.Second }, "CollectorInterval"},

		{"итераций ноль", func(c *Config) { c.CollectorMaxIterations = 0 }, "CollectorMaxIterations"},
		{"итераций отрицательное число", func(c *Config) { c.CollectorMaxIterations = -1 }, "CollectorMaxIterations"},

		{"batch ratio ноль", func(c *Config) { c.CollectorBatchRatio = 0 }, "CollectorBatchRatio"},
		{"batch ratio отрицательный", func(c *Config) { c.CollectorBatchRatio = -0.1 }, "CollectorBatchRatio"},
		{"batch ratio больше 1", func(c *Config) { c.CollectorBatchRatio = 1.0001 }, "CollectorBatchRatio"},

		{"threshold ratio ноль", func(c *Config) { c.CollectorThresholdRatio = 0 }, "CollectorThresholdRatio"},
		{"threshold ratio отрицательный", func(c *Config) { c.CollectorThresholdRatio = -0.1 }, "CollectorThresholdRatio"},
		{"threshold ratio больше 1", func(c *Config) { c.CollectorThresholdRatio = 1.0001 }, "CollectorThresholdRatio"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			tt.modify(&cfg)

			assert.ErrorContains(t, cfg.Validate(), tt.wantErr)
		})
	}
}

func TestConfig_Validate_ReturnsAllErrors(t *testing.T) {
	// пустой конфиг нарушает все правила сразу
	cfg := Config{}

	err := cfg.Validate()

	assert.Error(t, err)
	assert.ErrorContains(t, err, "InitialBucketCount")
	assert.ErrorContains(t, err, "CollectorInterval")
	assert.ErrorContains(t, err, "CollectorMaxIterations")
	assert.ErrorContains(t, err, "CollectorBatchRatio")
	assert.ErrorContains(t, err, "CollectorThresholdRatio")
}
