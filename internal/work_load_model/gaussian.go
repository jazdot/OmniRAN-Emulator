package work_load_model

import (
	"math/rand"
)

// gaussianlDistribution calculates some random numbers from the Gaussian distribution
func gaussianlDistribution(sigma float64, length int, const_seed int) (distGaussian []uint) {
	distGaussian = make([]uint, length)

	for i := 1; i <= length; i++ {
		seed := int64(const_seed + i)
		r := rand.New(rand.NewSource(seed))
		// r.NormFloat64() returns a normally distributed float64 with mean 0 and standard deviation 1.
		// Multiplying by sigma scales it to standard deviation sigma.
		val := r.NormFloat64() * sigma
		if val < 0 {
			// Clamp negative values to 0 since workload parameters (like packet sizes or intervals) must be positive.
			val = 0
		}
		distGaussian[i-1] = uint(val)
	}
	return
}
