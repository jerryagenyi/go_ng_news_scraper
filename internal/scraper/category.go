// Package scraper implements the core scraping functionality for Nigerian news websites.
// This file specifically handles the extraction and storage of category information
// from news websites' category sitemaps, maintaining hierarchical relationships
// between categories where applicable.
package scraper

import (
	"database/sql"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/jerryagenyi/go_ng_news_scraper/internal/config"
)

type CategorySitemap struct {
	XMLName xml.Name `xml:"urlset"`
	URLs    []struct {
		Loc     string `xml:"loc"`
		LastMod string `xml:"lastmod,omitempty"`
	} `xml:"url"`
}

type CategoryScraper struct {
	db     *sql.DB
	config config.WebsiteConfig
	client *http.Client
}

func NewCategoryScraper(db *sql.DB, config config.WebsiteConfig) *CategoryScraper {
	return &CategoryScraper{
		db:     db,
		config: config,
		client: &http.Client{
			Timeout: time.Duration(config.Timeout) * time.Second,
		},
	}
}

func (cs *CategoryScraper) ScrapeCategories() error {
	log.Printf("Starting category scraping for %s", cs.config.Name)
	log.Printf("Fetching categories from: %s", cs.config.CategorySitemapURL)

	resp, err := cs.client.Get(cs.config.CategorySitemapURL)
	if err != nil {
		return fmt.Errorf("failed to fetch category sitemap: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	var sitemap CategorySitemap
	if err := xml.Unmarshal(body, &sitemap); err != nil {
		return fmt.Errorf("failed to parse XML: %w", err)
	}

	log.Printf("Found %d categories in sitemap", len(sitemap.URLs))

	tx, err := cs.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	stmt, err := tx.Prepare(`
    INSERT INTO go_categories (
        website_id, 
        name, 
        slug, 
        url, 
        parent_id,
        created_at
    )
    VALUES ($1, $2, $3, $4, $5, NOW())
    ON CONFLICT (website_id, slug) 
    DO UPDATE SET 
        name = $2, 
        url = $4,
        parent_id = $5
    RETURNING id
`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, url := range sitemap.URLs {
		// Extract category name and slug from URL
		name, slug := extractCategoryInfo(url.Loc)
		log.Printf("Processing category: %s (slug: %s)", name, slug)

		parentID, err := cs.findParentID(tx, slug)
		if err != nil {
			log.Printf("Error finding parent for %s: %v", slug, err)
			continue
		}

		var id int
		err = stmt.QueryRow(
			cs.config.ID,
			name,
			slug,
			url.Loc,
			sql.NullInt32{Int32: int32(parentID), Valid: parentID > 0},
		).Scan(&id)

		if err != nil {
			log.Printf("Error inserting category %s: %v", url.Loc, err)
			continue
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	log.Printf("Successfully processed categories for %s", cs.config.Name)
	return nil
}

// extractCategoryInfo parses category information from the URL
// Example URL: https://blueprint.ng/category/world-stage/
func extractCategoryInfo(url string) (name, slug string) {
	// Remove trailing slash if present
	if url[len(url)-1] == '/' {
		url = url[:len(url)-1]
	}

	// Split by "category/"
	parts := strings.Split(url, "category/")
	if len(parts) != 2 {
		return "Unknown", "unknown"
	}

	// Get the category slug
	slug = parts[1]

	// Convert slug to readable name
	name = strings.Title(strings.ReplaceAll(slug, "-", " "))

	return name, slug
}

// Add new method to handle parent-child relationships
func (cs *CategoryScraper) findParentID(tx *sql.Tx, currentSlug string) (int, error) {
	// If slug contains '/', it has a parent
	parts := strings.Split(currentSlug, "/")
	if len(parts) == 1 {
		return 0, nil // No parent
	}

	// Parent slug is everything before the last '/'
	parentSlug := strings.Join(parts[:len(parts)-1], "/")
	var parentID int
	err := tx.QueryRow(`
        SELECT id FROM go_categories 
        WHERE website_id = $1 AND slug = $2
    `, cs.config.ID, parentSlug).Scan(&parentID)

	if err == sql.ErrNoRows {
		return 0, nil
	}
	return parentID, err
}
