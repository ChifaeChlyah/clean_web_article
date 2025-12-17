package main

import (
	"net/http"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/gin-gonic/gin"
)

const API_KEY = "demo_12345" // clé API
func looksLikeContent(line string) bool {
	// phrase assez longue
	if len(line) < 50 {
		return false
	}

	// doit contenir au moins une ponctuation classique
	if !strings.ContainsAny(line, ".?!") {
		return false
	}

	// au moins 8 mots
	if len(strings.Fields(line)) < 8 {
		return false
	}

	return true
}

func isBoilerplate(line string) bool {
	blacklist := []string{
		"sign up",
		"newsletter",
		"all rights reserved",
		"privacy policy",
		"terms of use",
		"download the app",
		"©",
	}

	l := strings.ToLower(line)
	for _, w := range blacklist {
		if strings.Contains(l, w) {
			return true
		}
	}
	return false
}

func cleanForLLM(raw string) string {
	lines := strings.Split(raw, "\n")
	seen := make(map[string]bool)
	var cleaned []string

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if !looksLikeContent(line) {
			continue
		}
		if isBoilerplate(line) {
			continue
		}
		if seen[line] {
			continue
		}

		seen[line] = true
		cleaned = append(cleaned, line)
	}

	text := strings.Join(cleaned, "\n\n")
	text = strings.TrimSpace(text)

	// limite raisonnable pour LLM
	words := strings.Fields(text)
	if len(words) > 1500 {
		text = strings.Join(words[:1500], " ") + "..."
	}

	return text
}

func extractHandler(c *gin.Context) {
	// Vérification de la clé API
	key := c.Query("key")
	if key != API_KEY {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or missing API key"})
		return
	}

	// Lire l'URL
	url := c.Query("url")
	if url == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing url parameter"})
		return
	}

	resp, err := http.Get(url)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch url"})
		return
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to parse page"})
		return
	}

	title := doc.Find("title").First().Text()
	var paragraphs []string

	doc.Find("p").Each(func(i int, s *goquery.Selection) {
		text := strings.TrimSpace(s.Text())
		if text != "" {
			paragraphs = append(paragraphs, text)
		}
	})

	rawText := strings.Join(paragraphs, "\n")
	cleanText := cleanForLLM(rawText)

	tokens := len(strings.Fields(cleanText)) // estimation simple

	c.JSON(200, gin.H{
		"title":           title,
		"clean_text":      cleanText,
		"tokens_estimate": tokens,
	})
}

func main() {
	router := gin.Default()
	router.GET("/extract", extractHandler)
	router.Run(":8080") // Render utilise ça
}
