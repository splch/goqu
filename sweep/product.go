package sweep

// product is the Cartesian product of multiple sweeps.
type product struct {
	inner  []Sweep
	params []string
	points []map[string]float64
}

// Product returns a sweep that is the Cartesian product of the given sweeps.
// If any inner sweep has length 0, the result is empty.
func Product(sweeps ...Sweep) Sweep {
	if len(sweeps) == 0 {
		return &product{}
	}

	// Collect all param names.
	var params []string
	seen := make(map[string]bool)
	for _, s := range sweeps {
		for _, p := range s.Params() {
			if !seen[p] {
				seen[p] = true
				params = append(params, p)
			}
		}
	}

	// Fold: start with [{}], expand per inner sweep.
	results := []map[string]float64{{}}
	for _, s := range sweeps {
		resolved := s.Resolve()
		if len(resolved) == 0 {
			return &product{inner: sweeps, params: params}
		}
		next := make([]map[string]float64, 0, len(results)*len(resolved))
		for _, existing := range results {
			for _, point := range resolved {
				merged := make(map[string]float64, len(existing)+len(point))
				for k, v := range existing {
					merged[k] = v
				}
				for k, v := range point {
					merged[k] = v
				}
				next = append(next, merged)
			}
		}
		results = next
	}

	return &product{inner: sweeps, params: params, points: results}
}

func (p *product) Len() int                      { return len(p.points) }
func (p *product) Params() []string              { return p.params }
func (p *product) Resolve() []map[string]float64 { return p.points }
