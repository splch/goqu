// Command gen builds the Goqu interactive quantum computing textbook from
// chapter source fragments and a shared layout template.
//
// Usage:
//
//	go run textbook/gen/main.go
package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"
)

// Chapter holds metadata for a single chapter from chapters.json.
type Chapter struct {
	Slug      string `json:"slug"`
	Title     string `json:"title"`
	Part      int    `json:"part"`
	PartTitle string `json:"partTitle"`
	Chapter   int    `json:"chapter"`
}

// Part groups chapters for sidebar navigation.
type Part struct {
	Number   int
	Title    string
	Chapters []ChapterNav
	Current  bool
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

func main() {
	genDir := "textbook/gen"
	outDir := "textbook"

	// Load chapter manifest.
	manifestData, err := os.ReadFile(filepath.Join(genDir, "chapters.json"))
	if err != nil {
		fatal("read chapters.json: %v", err)
	}
	var chapters []Chapter
	if err := json.Unmarshal(manifestData, &chapters); err != nil {
		fatal("parse chapters.json: %v", err)
	}

	// Parse layout template.
	layoutPath := filepath.Join(genDir, "layout.html")
	tmpl, err := template.ParseFiles(layoutPath)
	if err != nil {
		fatal("parse layout.html: %v", err)
	}

	// Build part groupings for sidebar.
	partsMap := map[int]*Part{}
	var partsOrder []int
	for _, ch := range chapters {
		p, ok := partsMap[ch.Part]
		if !ok {
			p = &Part{Number: ch.Part, Title: ch.PartTitle}
			partsMap[ch.Part] = p
			partsOrder = append(partsOrder, ch.Part)
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

	// Copy static assets.
	copyFile(filepath.Join(genDir, "style.css"), filepath.Join(outDir, "style.css"))
	copyDir(filepath.Join(genDir, "js"), filepath.Join(outDir, "js"))

	// Generate index page.
	indexContent, err := os.ReadFile(filepath.Join(genDir, "index.html"))
	if err != nil {
		fatal("read index.html: %v", err)
	}
	indexData := PageData{
		Title:   "Goqu Quantum Computing Textbook",
		Content: template.HTML(indexContent),
		Parts:   allParts,
		IsIndex: true,
	}
	writeTemplate(tmpl, filepath.Join(outDir, "index.html"), indexData)

	// Generate each chapter.
	if err := os.MkdirAll(filepath.Join(outDir, "chapters"), 0o755); err != nil {
		fatal("mkdir chapters: %v", err)
	}

	for i, ch := range chapters {
		srcPath := filepath.Join(genDir, "chapters", ch.Slug+".html")
		content, err := os.ReadFile(srcPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warn: skip %s: %v\n", ch.Slug, err)
			continue
		}

		// Strip front-matter comment if present.
		body := stripFrontMatter(string(content))

		// Build navigation parts with current markers.
		navParts := markCurrent(allParts, ch.Part, ch.Chapter)

		// Prev/next links.
		var prev, next *ChapterNav
		if i > 0 {
			prev = &ChapterNav{
				Slug:    chapters[i-1].Slug,
				Title:   chapters[i-1].Title,
				Chapter: chapters[i-1].Chapter,
			}
		}
		if i < len(chapters)-1 {
			next = &ChapterNav{
				Slug:    chapters[i+1].Slug,
				Title:   chapters[i+1].Title,
				Chapter: chapters[i+1].Chapter,
			}
		}

		data := PageData{
			Title:     ch.Title,
			Chapter:   ch.Chapter,
			Part:      ch.Part,
			PartTitle: ch.PartTitle,
			Content:   template.HTML(body),
			Parts:     navParts,
			Prev:      prev,
			Next:      next,
			Breadcrumb: []BreadcrumbItem{
				{Label: "Home", Href: "../index.html"},
				{Label: fmt.Sprintf("Part %d", ch.Part), Href: "../index.html#part-" + fmt.Sprint(ch.Part)},
			},
		}

		outPath := filepath.Join(outDir, "chapters", ch.Slug+".html")
		writeTemplate(tmpl, outPath, data)
		fmt.Printf("generated %s\n", outPath)
	}

	fmt.Printf("done: %d chapters\n", len(chapters))
}

// stripFrontMatter removes a leading <!-- ... --> comment block.
func stripFrontMatter(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "<!--") {
		if idx := strings.Index(s, "-->"); idx != -1 {
			s = strings.TrimSpace(s[idx+3:])
		}
	}
	return s
}

// markCurrent returns a copy of parts with Current flags set.
func markCurrent(parts []Part, partNum, chapNum int) []Part {
	out := make([]Part, len(parts))
	for i, p := range parts {
		out[i] = Part{
			Number:  p.Number,
			Title:   p.Title,
			Current: p.Number == partNum,
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

func writeTemplate(tmpl *template.Template, path string, data PageData) {
	f, err := os.Create(path)
	if err != nil {
		fatal("create %s: %v", path, err)
	}
	defer func() { _ = f.Close() }()
	if err := tmpl.Execute(f, data); err != nil {
		fatal("execute template for %s: %v", path, err)
	}
}

func copyFile(src, dst string) {
	data, err := os.ReadFile(src)
	if err != nil {
		fatal("read %s: %v", src, err)
	}
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		fatal("write %s: %v", dst, err)
	}
}

func copyDir(src, dst string) {
	if err := os.RemoveAll(dst); err != nil {
		fatal("remove %s: %v", dst, err)
	}
	err := filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
	if err != nil {
		fatal("copy dir %s -> %s: %v", src, dst, err)
	}
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "gen: "+format+"\n", args...)
	os.Exit(1)
}
