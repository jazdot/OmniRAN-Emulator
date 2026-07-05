package work_load_model

import (
	"math/rand"
)

// ExponentialDistribution calculates some random numbers from the Exponential distribution
func ExponentialDistribution(mean float64, length int, const_seed int) (distExpo []uint) {
	distExpo = make([]uint, length)

	for i := 1; i <= length; i++ {
		seed := int64(const_seed + i)
		r := rand.New(rand.NewSource(seed))
		// r.ExpFloat64() returns an exponentially distributed float64 with standard mean of 1.0.
		// Multiplying by the desired mean scales it correctly.
		distExpo[i-1] = uint(r.ExpFloat64() * mean)
	}
	return
}
