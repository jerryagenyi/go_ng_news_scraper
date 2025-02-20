// Package scraper implements the core scraping functionality for Nigerian news websites.
// This file handles the article scraping logic, including extraction of article content,
// metadata, and category relationships. It works in conjunction with the category
// scraper to maintain proper content organization.
package scraper

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/jerryagenyi/go_ng_news_scraper/internal/config" // Fix import path
)

// CSS selectors for Blueprint.ng
const (
	titleSelector     = "h1.entry-title"
	categorySelector  = "div.cat-links a"
	authorSelector    = "span.author.vcard a"
	publishedSelector = "time.entry-date.published"
	updatedSelector   = "time.updated"
	contentSelector   = "#post-695218 > div > p"
)

type ArticleScraper struct {
	db     *sql.DB
	config config.WebsiteConfig
	client *http.Client
}

func NewArticleScraper(db *sql.DB, config config.WebsiteConfig) *ArticleScraper {
	return &ArticleScraper{
		db:     db,
		config: config,
		client: &http.Client{},
	}
}

type Article struct {
	ID            int // Add this field
	Title         string
	Categories    []string // Category names for display
	Author        string
	PublishDate   time.Time
	UpdatedDate   time.Time
	Content       string
	URL           string
	CategoryIDs   []int    // Category IDs for database relations
	CategorySlugs []string // Category slugs for matching
	ContentHash   string   // Add this field
}

func (as *ArticleScraper) ScrapeArticle(url string) (*Article, error) {
	log.Printf("Scraping article: %s", url)

	resp, err := as.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch article: %w", err)
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	article := &Article{URL: url}

	// Extract title
	article.Title = strings.TrimSpace(doc.Find(titleSelector).Text())

	// Extract categories
	doc.Find(categorySelector).Each(func(i int, s *goquery.Selection) {
		categoryName := strings.TrimSpace(s.Text())
		if href, exists := s.Attr("href"); exists {
			// Extract slug from category URL
			// e.g., from "https://blueprint.ng/category/security/" get "security"
			// or "https://blueprint.ng/category/top-newspaper/" for Top Stories
			parts := strings.Split(href, "/category/")
			if len(parts) == 2 {
				slug := strings.TrimSuffix(parts[1], "/")
				article.CategorySlugs = append(article.CategorySlugs, slug)
				article.Categories = append(article.Categories, categoryName)
				log.Printf("Found category: %s (slug: %s)", categoryName, slug)
			}
		}
	})

	// Extract author
	article.Author = strings.TrimSpace(doc.Find(authorSelector).Text())

	// Extract dates
	if dateStr, exists := doc.Find(publishedSelector).Attr("datetime"); exists {
		if publishDate, err := time.Parse(time.RFC3339, dateStr); err == nil {
			article.PublishDate = publishDate
		}
	}

	if dateStr, exists := doc.Find(updatedSelector).Attr("datetime"); exists {
		if updatedDate, err := time.Parse(time.RFC3339, dateStr); err == nil {
			article.UpdatedDate = updatedDate
		}
	}

	// Extract content
	var contentBuilder strings.Builder
	doc.Find(contentSelector).Each(func(i int, s *goquery.Selection) {
		contentBuilder.WriteString(strings.TrimSpace(s.Text()))
		contentBuilder.WriteString("\n\n")
	})
	article.Content = strings.TrimSpace(contentBuilder.String())

	log.Printf("Found article: %s with %d categories", article.Title, len(article.Categories))
	return article, nil
}

func (as *ArticleScraper) hasChanged(existing, new *Article) bool {
	if existing.Title != new.Title ||
		existing.Author != new.Author ||
		existing.ContentHash != new.ContentHash || // Compare hashes instead of content
		!sameCategories(existing.CategorySlugs, new.CategorySlugs) {

		log.Printf("Changes detected in article: %s", new.Title)
		log.Printf("- Title changed: %v", existing.Title != new.Title)
		log.Printf("- Content changed: %v", existing.ContentHash != new.ContentHash)
		log.Printf("- Author changed: %v", existing.Author != new.Author)
		log.Printf("- Categories changed: %v", !sameCategories(existing.CategorySlugs, new.CategorySlugs))
		return true
	}

	log.Printf("No changes detected in article: %s", new.Title)
	return false
}

func sameCategories(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	seen := make(map[string]bool)
	for _, cat := range a {
		seen[cat] = true
	}
	for _, cat := range b {
		if !seen[cat] {
			return false
		}
	}
	return true
}

func (as *ArticleScraper) SaveArticle(article *Article) error {
	tx, err := as.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Calculate hash before saving
	article.ContentHash = CalculateContentHash(article.Content)

	// Get existing article if any
	var existing Article
	err = tx.QueryRow(`
		SELECT id, title, content, author, content_hash
		FROM go_articles 
		WHERE url = $1
	`, article.URL).Scan(&existing.ID, &existing.Title, &existing.Content,
		&existing.Author, &existing.ContentHash)

	if err == nil {
		// Get existing categories
		rows, err := tx.Query(`
			SELECT c.slug 
			FROM go_categories c
			JOIN go_article_categories ac ON c.id = ac.category_id
			WHERE ac.article_id = $1
		`, existing.ID)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var slug string
				rows.Scan(&slug)
				existing.CategorySlugs = append(existing.CategorySlugs, slug)
			}
		}

		// Check if anything meaningful has changed
		if !as.hasChanged(&existing, article) {
			log.Printf("No meaningful changes detected for: %s", article.Title)
			return nil
		}
		log.Printf("Changes detected, updating article: %s", article.Title)
	}

	// Proceed with upsert if article is new or has changed
	var articleID int
	err = tx.QueryRow(`
		INSERT INTO go_articles (
			website_id,
			title,
			content,
			content_hash,  -- Add this
			author,
			publish_date,
			last_updated,
			url,
			created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW())
		ON CONFLICT (url) DO UPDATE SET
			title = $2,
			content = $3,
			content_hash = $4,  -- Add this
			author = $5,
			publish_date = $6,
			last_updated = $7
		RETURNING id
	`, as.config.ID, article.Title, article.Content, article.ContentHash,
		article.Author, article.PublishDate, article.UpdatedDate,
		article.URL).Scan(&articleID)

	if err != nil {
		return fmt.Errorf("failed to upsert article: %w", err)
	}

	// Only update categories if the article was changed or is new
	// First, delete existing category relationships
	_, err = tx.Exec(`
		DELETE FROM go_article_categories 
		WHERE article_id = $1
	`, articleID)

	if err != nil {
		return fmt.Errorf("failed to clear existing categories: %w", err)
	}

	// Update category relationships to use slugs
	for i, slug := range article.CategorySlugs {
		var categoryID int
		err := tx.QueryRow(`
			SELECT id FROM go_categories 
			WHERE website_id = $1 AND slug = $2
		`, as.config.ID, slug).Scan(&categoryID)

		if err != nil {
			log.Printf("Category not found for slug '%s' (display name: '%s')",
				slug, article.Categories[i])
			continue
		}

		_, err = tx.Exec(`
			INSERT INTO go_article_categories (article_id, category_id)
			VALUES ($1, $2)
			ON CONFLICT DO NOTHING
		`, articleID, categoryID)

		if err != nil {
			log.Printf("Failed to link category %s: %v", article.Categories[i], err)
		}
	}

	return tx.Commit()
}

func CalculateContentHash(content string) string {
	hasher := sha256.New()
	hasher.Write([]byte(content))
	return hex.EncodeToString(hasher.Sum(nil))
}
