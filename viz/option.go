package viz

// Option configures visualization rendering.
type Option func(*config)

type config struct {
	style  *Style
	width  float64
	height float64
	title  string
	sorted bool
}

func defaultConfig() *config {
	return &config{
		style:  DefaultStyle(),
		width:  600,
		height: 400,
		sorted: true,
	}
}

func applyOpts(opts []Option) *config {
	cfg := defaultConfig()
	for _, o := range opts {
		o(cfg)
	}
	return cfg
}

// WithStyle sets the rendering style.
func WithStyle(s *Style) Option {
	return func(c *config) {
		if s != nil {
			c.style = s
		}
	}
}

// WithSize sets the SVG width and height in pixels.
func WithSize(width, height float64) Option {
	return func(c *config) {
		if width > 0 {
			c.width = width
		}
		if height > 0 {
			c.height = height
		}
	}
}

// WithTitle sets an optional title displayed above the plot.
func WithTitle(title string) Option {
	return func(c *config) {
		c.title = title
	}
}

// WithSorted controls whether histogram bars are sorted by bitstring.
// Default is true.
func WithSorted(sorted bool) Option {
	return func(c *config) {
		c.sorted = sorted
	}
}
