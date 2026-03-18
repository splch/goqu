package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRoman(t *testing.T) {
	tests := []struct {
		n    int
		want string
	}{
		{1, "I"}, {2, "II"}, {3, "III"}, {4, "IV"}, {5, "V"},
		{6, "VI"}, {7, "VII"}, {8, "VIII"}, {9, "IX"}, {10, "X"},
		{11, "XI"}, {12, "XII"}, {0, "0"}, {13, "13"},
	}
	for _, tt := range tests {
		if got := roman(tt.n); got != tt.want {
			t.Errorf("roman(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}

func TestMarkCurrent(t *testing.T) {
	parts := []Part{
		{Number: 1, Title: "Part 1", Chapters: []ChapterNav{
			{Slug: "01-foo", Chapter: 1},
			{Slug: "02-bar", Chapter: 2},
		}},
		{Number: 2, Title: "Part 2", Chapters: []ChapterNav{
			{Slug: "03-baz", Chapter: 3},
		}},
	}

	result := markCurrent(parts, 1, 2)

	if !result[0].Current {
		t.Error("part 1 should be current")
	}
	if result[1].Current {
		t.Error("part 2 should not be current")
	}
	if result[0].Chapters[0].Current {
		t.Error("chapter 1 should not be current")
	}
	if !result[0].Chapters[1].Current {
		t.Error("chapter 2 should be current")
	}
}

func TestGenerate(t *testing.T) {
	outDir := t.TempDir()
	if err := generate(outDir); err != nil {
		t.Fatalf("generate: %v", err)
	}

	// Verify index.html exists and contains generated chapter listings.
	indexHTML, err := os.ReadFile(filepath.Join(outDir, "index.html"))
	if err != nil {
		t.Fatalf("read index.html: %v", err)
	}
	idx := string(indexHTML)
	if !strings.Contains(idx, "Part I:") {
		t.Error("index.html missing Roman numeral part headings")
	}
	if !strings.Contains(idx, "What Is Computation?") {
		t.Error("index.html missing chapter 1 title")
	}
	if !strings.Contains(idx, "What Comes Next") {
		t.Error("index.html missing chapter 42 title")
	}

	// Verify style.css was written.
	if _, err := os.Stat(filepath.Join(outDir, "style.css")); err != nil {
		t.Error("style.css not generated")
	}

	// Verify chapter pages were generated.
	chapterPath := filepath.Join(outDir, "chapters", "01-computation.html")
	chapterHTML, err := os.ReadFile(chapterPath)
	if err != nil {
		t.Fatalf("read chapter 1: %v", err)
	}
	ch := string(chapterHTML)
	if !strings.Contains(ch, "Chapter 1: What Is Computation?") {
		t.Error("chapter 1 missing heading")
	}
	if !strings.Contains(ch, `aria-current="page"`) {
		t.Error("chapter 1 missing current page marker in sidebar")
	}
	// Verify prev/next navigation.
	if !strings.Contains(ch, "02-math-of-information.html") {
		t.Error("chapter 1 missing next link")
	}
}
