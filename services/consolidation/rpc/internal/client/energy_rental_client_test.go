package client

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidatePlatformData(t *testing.T) {
	data := &ItrxPlatformData{
		PlatformAvailEnergy: 100000,
		PlatformMaxEnergy:   50000,
		MinimumOrderEnergy:  1000,
		MaximumOrderEnergy:  80000,
		Balance:             1,
	}

	require.Error(t, ValidatePlatformData(data, 0))
	require.Error(t, ValidatePlatformData(data, 10))      // below minimum
	require.Error(t, ValidatePlatformData(data, 90000))   // above maximum
	require.Error(t, ValidatePlatformData(data, 60000))   // above platform max
	require.NoError(t, ValidatePlatformData(data, 32000)) // ok
}
