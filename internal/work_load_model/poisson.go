package work_load_model

import (
	"math"
	"math/rand"
)

// PoissonDistribution calculates some random numbers from the Poisson distribution
func PoissonDistribution(mean float64, length int, const_seed int) (distPoisson []uint) {
	distPoisson = make([]uint, length)

	for i := 1; i <= length; i++ {
		seed := int64(const_seed + i)
		r := rand.New(rand.NewSource(seed))

		// Knuth's algorithm for Poisson distribution:
		// L = e^(-mean), k = 0, p = 1
		// Repeat: k = k + 1, p = p * u (random [0,1))
		// Until p <= L, return k - 1
		L := math.Exp(-mean)
		
		// If L == 0 (due to floating point underflow when mean is large, e.g., mean >= 745),
		// we use the Gaussian approximation of the Poisson distribution:
		// Poisson(lambda) is approximated by Normal(mu = lambda, sigma = sqrt(lambda))
		if L == 0 {
			val := r.NormFloat64()*math.Sqrt(mean) + mean
			if val < 0 {
				val = 0
			}
			distPoisson[i-1] = uint(math.Round(val))
		} else {
			k := 0
			p := 1.0
			for p > L {
				k++
				p *= r.Float64()
			}
			distPoisson[i-1] = uint(k - 1)
		}
	}
	return
}
