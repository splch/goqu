package mitigation

import (
	"context"
	"fmt"
	"math"
)

// BasisExecutor prepares a computational basis state and measures it.
// basisState is the integer index of the basis state to prepare (0 to 2^n - 1).
// shots is the number of measurement repetitions.
// Returns a map from bitstring to count.
type BasisExecutor func(ctx context.Context, basisState int, shots int) (map[string]int, error)

// ReadoutCalibration holds the confusion matrix and its inverse for
// measurement error mitigation.
type ReadoutCalibration struct {
	numQubits int
	dim       int         // 2^numQubits
	matrix    [][]float64 // dim×dim confusion matrix: matrix[measured][prepared]
	inverse   [][]float64 // dim×dim inverse confusion matrix
}

// CalibrateReadout builds a full confusion matrix by preparing all 2^n
// computational basis states and measuring. Practical for n ≤ 10.
func CalibrateReadout(ctx context.Context, numQubits, shots int, exec BasisExecutor) (*ReadoutCalibration, error) {
	if numQubits < 1 {
		return nil, fmt.Errorf("mitigation.CalibrateReadout: numQubits must be >= 1")
	}
	if shots < 1 {
		return nil, fmt.Errorf("mitigation.CalibrateReadout: shots must be >= 1")
	}

	dim := 1 << numQubits
	matrix := make([][]float64, dim)
	for i := range dim {
		matrix[i] = make([]float64, dim)
	}

	for prepared := range dim {
		counts, err := exec(ctx, prepared, shots)
		if err != nil {
			return nil, fmt.Errorf("mitigation.CalibrateReadout: basis state %d: %w", prepared, err)
		}

		total := 0
		for _, c := range counts {
			total += c
		}
		if total == 0 {
			continue
		}

		for bs, c := range counts {
			measured := bitstringToInt(bs)
			matrix[measured][prepared] = float64(c) / float64(total)
		}
	}

	inv, err := invertMatrix(matrix, dim)
	if err != nil {
		return nil, fmt.Errorf("mitigation.CalibrateReadout: invert confusion matrix: %w", err)
	}

	return &ReadoutCalibration{
		numQubits: numQubits,
		dim:       dim,
		matrix:    matrix,
		inverse:   inv,
	}, nil
}

// CalibrateReadoutPerQubit builds a confusion matrix from per-qubit
// calibrations. Only requires 2 basis state preparations (|0...0⟩ and |1...1⟩)
// and constructs the full confusion matrix as a tensor product of 2×2 matrices.
// Scales to large qubit counts.
func CalibrateReadoutPerQubit(ctx context.Context, numQubits, shots int, exec BasisExecutor) (*ReadoutCalibration, error) {
	if numQubits < 1 {
		return nil, fmt.Errorf("mitigation.CalibrateReadoutPerQubit: numQubits must be >= 1")
	}
	if shots < 1 {
		return nil, fmt.Errorf("mitigation.CalibrateReadoutPerQubit: shots must be >= 1")
	}

	// Prepare all-zeros state.
	counts0, err := exec(ctx, 0, shots)
	if err != nil {
		return nil, fmt.Errorf("mitigation.CalibrateReadoutPerQubit: all-zeros: %w", err)
	}

	// Prepare all-ones state.
	allOnes := (1 << numQubits) - 1
	counts1, err := exec(ctx, allOnes, shots)
	if err != nil {
		return nil, fmt.Errorf("mitigation.CalibrateReadoutPerQubit: all-ones: %w", err)
	}

	// Extract per-qubit error rates.
	// For all-zeros: count how often each qubit reads 1 (= P01).
	// For all-ones: count how often each qubit reads 0 (= P10).
	total0 := 0
	for _, c := range counts0 {
		total0 += c
	}
	total1 := 0
	for _, c := range counts1 {
		total1 += c
	}

	type qubitCal struct {
		p00, p01, p10, p11 float64
	}
	qcals := make([]qubitCal, numQubits)

	if total0 > 0 {
		for bs, c := range counts0 {
			idx := bitstringToInt(bs)
			frac := float64(c) / float64(total0)
			for q := range numQubits {
				if (idx>>q)&1 == 1 {
					qcals[q].p01 += frac // prepared 0, measured 1
				} else {
					qcals[q].p00 += frac // prepared 0, measured 0
				}
			}
		}
	}

	if total1 > 0 {
		for bs, c := range counts1 {
			idx := bitstringToInt(bs)
			frac := float64(c) / float64(total1)
			for q := range numQubits {
				if (idx>>q)&1 == 1 {
					qcals[q].p11 += frac // prepared 1, measured 1
				} else {
					qcals[q].p10 += frac // prepared 1, measured 0
				}
			}
		}
	}

	// Build full confusion matrix via tensor product of 2×2 matrices.
	// qubit 0 is LSB of the bitstring index.
	dim := 1 << numQubits
	matrix := make([][]float64, dim)
	for i := range dim {
		matrix[i] = make([]float64, dim)
	}

	for measured := range dim {
		for prepared := range dim {
			prob := 1.0
			for q := range numQubits {
				mBit := (measured >> q) & 1
				pBit := (prepared >> q) & 1
				switch {
				case pBit == 0 && mBit == 0:
					prob *= qcals[q].p00
				case pBit == 0 && mBit == 1:
					prob *= qcals[q].p01
				case pBit == 1 && mBit == 0:
					prob *= qcals[q].p10
				case pBit == 1 && mBit == 1:
					prob *= qcals[q].p11
				}
			}
			matrix[measured][prepared] = prob
		}
	}

	inv, err2 := invertMatrix(matrix, dim)
	if err2 != nil {
		return nil, fmt.Errorf("mitigation.CalibrateReadoutPerQubit: invert confusion matrix: %w", err2)
	}

	return &ReadoutCalibration{
		numQubits: numQubits,
		dim:       dim,
		matrix:    matrix,
		inverse:   inv,
	}, nil
}

// CorrectCounts applies the inverse confusion matrix to raw measurement counts.
// Negative corrected values are clipped to zero and the result is renormalized.
func (cal *ReadoutCalibration) CorrectCounts(counts map[string]int) map[string]int {
	// Convert counts to probability vector.
	total := 0
	for _, c := range counts {
		total += c
	}
	if total == 0 {
		return counts
	}

	probs := make([]float64, cal.dim)
	for bs, c := range counts {
		idx := bitstringToInt(bs)
		if idx < cal.dim {
			probs[idx] = float64(c) / float64(total)
		}
	}

	// Apply inverse.
	corrected := make([]float64, cal.dim)
	for i := range cal.dim {
		for j := range cal.dim {
			corrected[i] += cal.inverse[i][j] * probs[j]
		}
	}

	// Clip negatives and renormalize.
	sum := 0.0
	for i := range corrected {
		if corrected[i] < 0 {
			corrected[i] = 0
		}
		sum += corrected[i]
	}

	result := make(map[string]int)
	if sum > 0 {
		for i := range cal.dim {
			c := int(math.Round(corrected[i] / sum * float64(total)))
			if c > 0 {
				result[intToBitstring(i, cal.numQubits)] = c
			}
		}
	}
	return result
}

// CorrectProbabilities applies the inverse confusion matrix to a probability distribution.
// Negative values are clipped to zero and the result is renormalized.
func (cal *ReadoutCalibration) CorrectProbabilities(probs map[string]float64) map[string]float64 {
	vec := make([]float64, cal.dim)
	for bs, p := range probs {
		idx := bitstringToInt(bs)
		if idx < cal.dim {
			vec[idx] = p
		}
	}

	corrected := make([]float64, cal.dim)
	for i := range cal.dim {
		for j := range cal.dim {
			corrected[i] += cal.inverse[i][j] * vec[j]
		}
	}

	// Clip and renormalize.
	sum := 0.0
	for i := range corrected {
		if corrected[i] < 0 {
			corrected[i] = 0
		}
		sum += corrected[i]
	}

	result := make(map[string]float64)
	if sum > 0 {
		for i := range cal.dim {
			p := corrected[i] / sum
			if p > 1e-15 {
				result[intToBitstring(i, cal.numQubits)] = p
			}
		}
	}
	return result
}

// invertMatrix inverts a square matrix using Gaussian elimination with partial pivoting.
func invertMatrix(m [][]float64, n int) ([][]float64, error) {
	// Build augmented matrix [M | I].
	aug := make([][]float64, n)
	for i := range n {
		aug[i] = make([]float64, 2*n)
		copy(aug[i], m[i])
		aug[i][n+i] = 1
	}

	// Forward elimination.
	for col := range n {
		// Partial pivoting.
		maxVal := math.Abs(aug[col][col])
		maxRow := col
		for row := col + 1; row < n; row++ {
			if v := math.Abs(aug[row][col]); v > maxVal {
				maxVal = v
				maxRow = row
			}
		}
		if maxVal < 1e-15 {
			return nil, fmt.Errorf("singular matrix at column %d", col)
		}
		aug[col], aug[maxRow] = aug[maxRow], aug[col]

		pivot := aug[col][col]
		for j := range 2 * n {
			aug[col][j] /= pivot
		}

		for row := range n {
			if row == col {
				continue
			}
			factor := aug[row][col]
			for j := range 2 * n {
				aug[row][j] -= factor * aug[col][j]
			}
		}
	}

	// Extract inverse.
	inv := make([][]float64, n)
	for i := range n {
		inv[i] = make([]float64, n)
		copy(inv[i], aug[i][n:])
	}
	return inv, nil
}

// bitstringToInt converts a bitstring like "01" to an integer.
// Leftmost character is the highest-index qubit (MSB-first convention).
func bitstringToInt(bs string) int {
	val := 0
	n := len(bs)
	for i, ch := range bs {
		if ch == '1' {
			val |= 1 << (n - 1 - i)
		}
	}
	return val
}

// intToBitstring converts an integer to a bitstring of given width.
// MSB-first convention: leftmost character is the highest-index qubit.
func intToBitstring(val, numQubits int) string {
	bs := make([]byte, numQubits)
	for i := range numQubits {
		if val&(1<<i) != 0 {
			bs[numQubits-1-i] = '1'
		} else {
			bs[numQubits-1-i] = '0'
		}
	}
	return string(bs)
}
