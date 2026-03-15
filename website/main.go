// Package main runs the Goqu quantum computing education website.
package main

import (
	"embed"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
)

//go:embed templates/*.html templates/lessons/*.html
var templateFS embed.FS

//go:embed static/*
var staticFS embed.FS

var templates *template.Template

func main() {
	var err error
	templates, err = template.New("").Funcs(template.FuncMap{
		"safeHTML": func(s string) template.HTML { return template.HTML(s) },
		"seq": func(n int) []int {
			s := make([]int, n)
			for i := range s {
				s[i] = i
			}
			return s
		},
		"add": func(a, b int) int { return a + b },
		"sub": func(a, b int) int { return a - b },
	}).ParseFS(templateFS, "templates/*.html", "templates/lessons/*.html")
	if err != nil {
		log.Fatalf("parse templates: %v", err)
	}

	mux := http.NewServeMux()

	// Static files
	mux.Handle("GET /static/", http.FileServerFS(staticFS))

	// Pages
	mux.HandleFunc("GET /{$}", handleIndex)
	mux.HandleFunc("GET /lesson/{id}", handleLesson)
	mux.HandleFunc("GET /sandbox", handleSandbox)

	// API
	mux.HandleFunc("POST /api/simulate", handleSimulate)
	mux.HandleFunc("POST /api/sandbox", handleSandboxSimulate)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	addr := fmt.Sprintf(":%s", port)
	log.Printf("Goqu Education Server listening on http://localhost%s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}
