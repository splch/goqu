package sweep

import "fmt"

// zip merges sweeps element-wise at each index.
type zip struct {
	inner  []Sweep
	params []string
	points []map[string]float64
}

// Zip returns a sweep that merges the given sweeps element-wise.
// All sweeps must have the same length; returns an error otherwise.
func Zip(sweeps ...Sweep) (Sweep, error) {
	if len(sweeps) == 0 {
		return &zip{}, nil
	}

	n := sweeps[0].Len()
	for i := 1; i < len(sweeps); i++ {
		if sweeps[i].Len() != n {
			return nil, fmt.Errorf("sweep.Zip: length mismatch: sweep 0 has %d points, sweep %d has %d", n, i, sweeps[i].Len())
		}
	}

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

	// Resolve each sweep and merge element-wise.
	resolved := make([][]map[string]float64, len(sweeps))
	for i, s := range sweeps {
		resolved[i] = s.Resolve()
	}

	points := make([]map[string]float64, n)
	for i := range n {
		merged := make(map[string]float64)
		for _, r := range resolved {
			for k, v := range r[i] {
				merged[k] = v
			}
		}
		points[i] = merged
	}

	return &zip{inner: sweeps, params: params, points: points}, nil
}

// MustZip is like Zip but panics on length mismatch.
func MustZip(sweeps ...Sweep) Sweep {
	s, err := Zip(sweeps...)
	if err != nil {
		panic(err)
	}
	return s
}

func (z *zip) Len() int                      { return len(z.points) }
func (z *zip) Params() []string              { return z.params }
func (z *zip) Resolve() []map[string]float64 { return z.points }
