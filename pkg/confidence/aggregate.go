package confidence

import "math"

// logit maps (0, 1) → (-∞, ∞). Input is assumed already clipped.
func logit(p float64) float64 {
	return math.Log(p / (1 - p))
}

// sigma is the logistic function, inverse of logit.
func sigma(z float64) float64 {
	// Numerically stable form for |z| large.
	if z >= 0 {
		ez := math.Exp(-z)
		return 1 / (1 + ez)
	}
	ez := math.Exp(z)
	return ez / (1 + ez)
}

// clip bounds x to [ε, 1-ε].
func clip(x, eps float64) float64 {
	switch {
	case x < eps:
		return eps
	case x > 1-eps:
		return 1 - eps
	default:
		return x
	}
}

// aggregate combines factors in log-odds space. See docs/confidence.md §5.1.
//
// z = β₀ + β_E·ℓ(E) + β_R·ℓ(R) + β_S·ℓ(S) + β_T·ℓ(T) + β_K·ℓ(K)
// C_raw = σ(z)
func aggregate(b Breakdown, cfg Config) float64 {
	eps := cfg.ClipEpsilon
	if eps <= 0 {
		eps = 1e-3
	}

	z := cfg.Beta0 +
		cfg.BetaE*logit(clip(b.Evidence, eps)) +
		cfg.BetaR*logit(clip(b.Robustness, eps)) +
		cfg.BetaS*logit(clip(b.Safety, eps)) +
		cfg.BetaT*logit(clip(b.TemporalStability, eps)) +
		cfg.BetaK*logit(clip(b.CoverageConcentration, eps))

	return sigma(z)
}
