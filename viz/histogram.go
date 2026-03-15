package viz

import (
	"fmt"
	"io"
	"math"
	"sort"
	"strings"
)

// Histogram returns an SVG bar chart of measurement counts.
func Histogram(counts map[string]int, opts ...Option) string {
	var sb strings.Builder
	_ = FprintHistogram(&sb, counts, opts...)
	return sb.String()
}

// FprintHistogram writes an SVG bar chart of measurement counts to w.
func FprintHistogram(w io.Writer, counts map[string]int, opts ...Option) error {
	labels := make([]string, 0, len(counts))
	values := make([]float64, 0, len(counts))
	for k, v := range counts {
		labels = append(labels, k)
		values = append(values, float64(v))
	}
	return renderHistogram(w, labels, values, false, applyOpts(opts))
}

// HistogramProb returns an SVG bar chart of measurement probabilities.
func HistogramProb(probs map[string]float64, opts ...Option) string {
	var sb strings.Builder
	_ = FprintHistogramProb(&sb, probs, opts...)
	return sb.String()
}

// FprintHistogramProb writes an SVG bar chart of measurement probabilities to w.
func FprintHistogramProb(w io.Writer, probs map[string]float64, opts ...Option) error {
	labels := make([]string, 0, len(probs))
	values := make([]float64, 0, len(probs))
	for k, v := range probs {
		labels = append(labels, k)
		values = append(values, v)
	}
	return renderHistogram(w, labels, values, true, applyOpts(opts))
}

func renderHistogram(w io.Writer, labels []string, values []float64, isProb bool, cfg *config) error {
	sty := cfg.style

	if len(labels) == 0 {
		_, err := io.WriteString(w, `<svg xmlns="http://www.w3.org/2000/svg"/>`)
		return err
	}

	// Sort labels and values together.
	if cfg.sorted {
		sortPaired(labels, values)
	}

	// Compute max value for Y-axis scaling.
	maxVal := 0.0
	for _, v := range values {
		if v > maxVal {
			maxVal = v
		}
	}
	if isProb {
		// Probability axis always goes to at least 1.0 but can be lower for cleaner ticks.
		if maxVal < 1.0 {
			maxVal = ceilNice(maxVal)
		} else {
			maxVal = 1.0
		}
	}

	niceMax, tickInterval := niceScale(maxVal, 5)
	if isProb && niceMax > 1.0 {
		niceMax = 1.0
		tickInterval = 0.2
	}

	// Layout geometry.
	leftMargin := sty.Padding + 50  // Y-axis labels
	rightMargin := sty.Padding      // right edge
	topMargin := sty.Padding + 10   // top edge (or title)
	bottomMargin := sty.Padding + 50 // X-axis labels

	if cfg.title != "" {
		topMargin += 20
	}

	plotW := cfg.width - leftMargin - rightMargin
	plotH := cfg.height - topMargin - bottomMargin

	n := len(labels)
	barGap := 0.2 // fraction of slot for gap
	slotW := plotW / float64(n)
	barW := slotW * (1 - barGap)

	var sb strings.Builder
	sb.WriteString(svgHeader(cfg.width, cfg.height, sty))

	// Title.
	if cfg.title != "" {
		fmt.Fprintf(&sb, `<text x="%.1f" y="%.1f" fill="%s" text-anchor="middle" font-size="%.0f" font-weight="bold">%s</text>`+"\n",
			cfg.width/2, sty.Padding+14, sty.TextColor, sty.FontSize+2, xmlEscape(cfg.title))
	}

	// Y-axis.
	fmt.Fprintf(&sb, `<line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="%s" stroke-width="1"/>`+"\n",
		leftMargin, topMargin, leftMargin, topMargin+plotH, sty.AxisColor)

	// X-axis.
	fmt.Fprintf(&sb, `<line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="%s" stroke-width="1"/>`+"\n",
		leftMargin, topMargin+plotH, leftMargin+plotW, topMargin+plotH, sty.AxisColor)

	// Y-axis ticks and grid lines.
	if tickInterval > 0 && niceMax > 0 {
		for tick := 0.0; tick <= niceMax+tickInterval*0.01; tick += tickInterval {
			y := topMargin + plotH - (tick/niceMax)*plotH
			// Grid line.
			fmt.Fprintf(&sb, `<line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="%s" stroke-width="0.5" stroke-dasharray="4,4"/>`+"\n",
				leftMargin, y, leftMargin+plotW, y, sty.GridColor)
			// Tick label.
			var label string
			if isProb {
				label = fmt.Sprintf("%.2f", tick)
			} else {
				label = fmt.Sprintf("%.0f", tick)
			}
			fmt.Fprintf(&sb, `<text x="%.1f" y="%.1f" fill="%s" text-anchor="end" dominant-baseline="middle" font-size="%.0f">%s</text>`+"\n",
				leftMargin-6, y, sty.TextColor, sty.FontSize-1, label)
		}
	}

	// Bars and X-axis labels.
	rotate := n > 8
	for i, v := range values {
		barH := 0.0
		if niceMax > 0 {
			barH = (v / niceMax) * plotH
		}
		x := leftMargin + float64(i)*slotW + (slotW-barW)/2
		y := topMargin + plotH - barH

		// Bar.
		fmt.Fprintf(&sb, `<rect x="%.1f" y="%.1f" width="%.1f" height="%.1f" fill="%s" stroke="%s" stroke-width="0.5"/>`+"\n",
			x, y, barW, barH, sty.BarFill, sty.BarStroke)

		// X-axis label.
		labelX := leftMargin + float64(i)*slotW + slotW/2
		labelY := topMargin + plotH + 14
		if rotate {
			fmt.Fprintf(&sb, `<text x="%.1f" y="%.1f" fill="%s" text-anchor="end" dominant-baseline="middle" font-size="%.0f" transform="rotate(-45,%.1f,%.1f)">%s</text>`+"\n",
				labelX, labelY, sty.TextColor, sty.FontSize-1, labelX, labelY, xmlEscape(labels[i]))
		} else {
			fmt.Fprintf(&sb, `<text x="%.1f" y="%.1f" fill="%s" text-anchor="middle" dominant-baseline="hanging" font-size="%.0f">%s</text>`+"\n",
				labelX, labelY, sty.TextColor, sty.FontSize-1, xmlEscape(labels[i]))
		}
	}

	sb.WriteString(svgFooter())
	_, err := io.WriteString(w, sb.String())
	return err
}

// sortPaired sorts labels lexicographically, keeping values in sync.
func sortPaired(labels []string, values []float64) {
	indices := make([]int, len(labels))
	for i := range indices {
		indices[i] = i
	}
	sort.Slice(indices, func(i, j int) bool {
		return labels[indices[i]] < labels[indices[j]]
	})
	sortedL := make([]string, len(labels))
	sortedV := make([]float64, len(values))
	for i, idx := range indices {
		sortedL[i] = labels[idx]
		sortedV[i] = values[idx]
	}
	copy(labels, sortedL)
	copy(values, sortedV)
}

// niceScale computes a "nice" max and tick interval for an axis.
func niceScale(maxVal float64, maxTicks int) (niceMax, tickInterval float64) {
	if maxVal <= 0 {
		return 1, 0.5
	}
	rawInterval := maxVal / float64(maxTicks)
	magnitude := math.Pow(10, math.Floor(math.Log10(rawInterval)))
	residual := rawInterval / magnitude
	var nice float64
	switch {
	case residual <= 1.5:
		nice = 1
	case residual <= 3:
		nice = 2
	case residual <= 7:
		nice = 5
	default:
		nice = 10
	}
	tickInterval = nice * magnitude
	niceMax = math.Ceil(maxVal/tickInterval) * tickInterval
	return
}

// ceilNice rounds a probability value up to a clean display ceiling.
func ceilNice(v float64) float64 {
	steps := []float64{0.1, 0.2, 0.25, 0.5, 0.75, 1.0}
	for _, s := range steps {
		if v <= s {
			return s
		}
	}
	return 1.0
}
