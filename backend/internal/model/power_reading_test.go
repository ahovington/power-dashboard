package model_test

import (
	"testing"

	"github.com/ahovingtonpower-dashboard/internal/model"
	"github.com/stretchr/testify/assert"
)

func TestPowerReading_PowerNet(t *testing.T) {
	r := &model.PowerReading{PowerProduced: 5000, PowerConsumed: 3000}
	assert.Equal(t, 2000, r.PowerNet())
}

func TestPowerReading_PowerNet_WhenConsumedExceedsProduced(t *testing.T) {
	r := &model.PowerReading{PowerProduced: 1000, PowerConsumed: 3000}
	assert.Equal(t, -2000, r.PowerNet(), "negative net means drawing from grid")
}
