package draw

// Style configures SVG rendering appearance.
type Style struct {
	ColWidth        float64
	RowHeight       float64
	GateWidth       float64
	GateHeight      float64
	Padding         float64
	WireColor       string
	Gate1QFill      string
	Gate2QFill      string
	MeasureFill     string
	ControlFill     string
	TextColor       string
	BackgroundColor string
	FontFamily      string
	FontSize        float64
}

// DefaultStyle returns a light-theme style.
func DefaultStyle() *Style {
	return &Style{
		ColWidth:        60,
		RowHeight:       50,
		GateWidth:       40,
		GateHeight:      30,
		Padding:         40,
		WireColor:       "#000000",
		Gate1QFill:      "#BDD7FF",
		Gate2QFill:      "#D4BBFF",
		MeasureFill:     "#FFDDAA",
		ControlFill:     "#000000",
		TextColor:       "#000000",
		BackgroundColor: "#FFFFFF",
		FontFamily:      "monospace",
		FontSize:        12,
	}
}

// DarkStyle returns a dark-theme style.
func DarkStyle() *Style {
	return &Style{
		ColWidth:        60,
		RowHeight:       50,
		GateWidth:       40,
		GateHeight:      30,
		Padding:         40,
		WireColor:       "#CCCCCC",
		Gate1QFill:      "#2A4D7F",
		Gate2QFill:      "#4A2D7F",
		MeasureFill:     "#7F5A2D",
		ControlFill:     "#CCCCCC",
		TextColor:       "#EEEEEE",
		BackgroundColor: "#1E1E1E",
		FontFamily:      "monospace",
		FontSize:        12,
	}
}
