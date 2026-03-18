// Command gen builds the Goqu interactive quantum computing textbook from
// chapter source fragments and a shared layout template.
//
// Usage:
//
//	go run textbook/gen/main.go
//	go run textbook/gen/main.go -out /tmp/textbook
package main

import (
	"bytes"
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io/fs"
	"os"
	"path/filepath"
)

//go:embed chapters.json
var chaptersJSON []byte

//go:embed layout.html
var layoutHTML string

//go:embed index.tmpl.html
var indexTmplHTML string

//go:embed style.css
var styleCSS []byte

//go:embed chapters
var chaptersFS embed.FS

// Chapter holds metadata for a single chapter from chapters.json.
type Chapter struct {
	Slug            string `json:"slug"`
	Title           string `json:"title"`
	Part            int    `json:"part"`
	PartTitle       string `json:"partTitle"`
	PartDescription string `json:"partDescription,omitempty"`
	Chapter         int    `json:"chapter"`
}

// Part groups chapters for sidebar navigation.
type Part struct {
	Number      int
	Title       string
	Description string
	Chapters    []ChapterNav
	Current     bool
}

// ChapterNav is a chapter reference used in navigation.
type ChapterNav struct {
	Slug    string
	Title   string
	Chapter int
	Current bool
}

// PageData is the template context for a chapter page.
type PageData struct {
	Title      string
	Chapter    int
	Part       int
	PartTitle  string
	Content    template.HTML
	Parts      []Part
	Prev       *ChapterNav
	Next       *ChapterNav
	Breadcrumb []BreadcrumbItem
	IsIndex    bool
}

// BreadcrumbItem is a single breadcrumb link.
type BreadcrumbItem struct {
	Label string
	Href  string
}

var romanNumerals = [...]string{"", "I", "II", "III", "IV", "V", "VI", "VII", "VIII", "IX", "X", "XI", "XII"}

func roman(n int) string {
	if n > 0 && n < len(romanNumerals) {
		return romanNumerals[n]
	}
	return fmt.Sprintf("%d", n)
}

func main() {
	outDir := flag.String("out", "textbook", "output directory")
	flag.Parse()
	if err := generate(*outDir); err != nil {
		fmt.Fprintf(os.Stderr, "gen: %v\n", err)
		os.Exit(1)
	}
}

func generate(outDir string) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", outDir, err)
	}

	// Load chapter manifest.
	var chapters []Chapter
	if err := json.Unmarshal(chaptersJSON, &chapters); err != nil {
		return fmt.Errorf("parse chapters.json: %w", err)
	}

	// Parse templates.
	funcMap := template.FuncMap{"roman": roman}
	layoutTmpl, err := template.New("layout").Funcs(funcMap).Parse(layoutHTML)
	if err != nil {
		return fmt.Errorf("parse layout.html: %w", err)
	}
	indexTmpl, err := template.New("index").Funcs(funcMap).Parse(indexTmplHTML)
	if err != nil {
		return fmt.Errorf("parse index.tmpl.html: %w", err)
	}

	// Build part groupings for sidebar.
	partsMap := map[int]*Part{}
	var partsOrder []int
	for _, ch := range chapters {
		p, ok := partsMap[ch.Part]
		if !ok {
			p = &Part{Number: ch.Part, Title: ch.PartTitle, Description: ch.PartDescription}
			partsMap[ch.Part] = p
			partsOrder = append(partsOrder, ch.Part)
		} else if p.Description == "" && ch.PartDescription != "" {
			p.Description = ch.PartDescription
		}
		p.Chapters = append(p.Chapters, ChapterNav{
			Slug:    ch.Slug,
			Title:   ch.Title,
			Chapter: ch.Chapter,
		})
	}
	allParts := make([]Part, 0, len(partsOrder))
	for _, n := range partsOrder {
		allParts = append(allParts, *partsMap[n])
	}

	// Write static assets.
	if err := os.WriteFile(filepath.Join(outDir, "style.css"), styleCSS, 0o644); err != nil {
		return fmt.Errorf("write style.css: %w", err)
	}

	// Generate index page.
	var indexBuf bytes.Buffer
	indexData := PageData{
		Title:   "Goqu Quantum Computing Textbook",
		Parts:   allParts,
		IsIndex: true,
	}
	if err := indexTmpl.Execute(&indexBuf, indexData); err != nil {
		return fmt.Errorf("execute index template: %w", err)
	}
	indexData.Content = template.HTML(indexBuf.String())
	if err := writeTemplate(layoutTmpl, filepath.Join(outDir, "index.html"), indexData); err != nil {
		return err
	}

	// Generate each chapter.
	if err := os.MkdirAll(filepath.Join(outDir, "chapters"), 0o755); err != nil {
		return fmt.Errorf("mkdir chapters: %w", err)
	}

	for i, ch := range chapters {
		content, err := fs.ReadFile(chaptersFS, "chapters/"+ch.Slug+".html")
		if err != nil {
			fmt.Fprintf(os.Stderr, "warn: skip %s: %v\n", ch.Slug, err)
			continue
		}

		navParts := markCurrent(allParts, ch.Part, ch.Chapter)

		var prev, next *ChapterNav
		if i > 0 {
			prev = &ChapterNav{Slug: chapters[i-1].Slug, Title: chapters[i-1].Title, Chapter: chapters[i-1].Chapter}
		}
		if i < len(chapters)-1 {
			next = &ChapterNav{Slug: chapters[i+1].Slug, Title: chapters[i+1].Title, Chapter: chapters[i+1].Chapter}
		}

		data := PageData{
			Title:     ch.Title,
			Chapter:   ch.Chapter,
			Part:      ch.Part,
			PartTitle: ch.PartTitle,
			Content:   template.HTML(content),
			Parts:     navParts,
			Prev:      prev,
			Next:      next,
			Breadcrumb: []BreadcrumbItem{
				{Label: "Home", Href: "../index.html"},
				{Label: fmt.Sprintf("Part %d", ch.Part), Href: "../index.html#part-" + fmt.Sprint(ch.Part)},
			},
		}

		outPath := filepath.Join(outDir, "chapters", ch.Slug+".html")
		if err := writeTemplate(layoutTmpl, outPath, data); err != nil {
			return err
		}
		fmt.Printf("generated %s\n", outPath)
	}

	fmt.Printf("done: %d chapters\n", len(chapters))
	return nil
}

// markCurrent returns a copy of parts with Current flags set.
func markCurrent(parts []Part, partNum, chapNum int) []Part {
	out := make([]Part, len(parts))
	for i, p := range parts {
		out[i] = Part{
			Number:      p.Number,
			Title:       p.Title,
			Description: p.Description,
			Current:     p.Number == partNum,
		}
		out[i].Chapters = make([]ChapterNav, len(p.Chapters))
		for j, ch := range p.Chapters {
			out[i].Chapters[j] = ChapterNav{
				Slug:    ch.Slug,
				Title:   ch.Title,
				Chapter: ch.Chapter,
				Current: ch.Chapter == chapNum,
			}
		}
	}
	return out
}

func writeTemplate(tmpl *template.Template, path string, data PageData) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()
	if err := tmpl.Execute(f, data); err != nil {
		return fmt.Errorf("execute template for %s: %w", path, err)
	}
	return nil
}
