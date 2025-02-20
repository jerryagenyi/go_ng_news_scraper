// Package scraper implements the core scraping functionality for Nigerian news websites.
// This file handles the article scraping logic, including extraction of article content,
// metadata, and category relationships. It works in conjunction with the category
// scraper to maintain proper content organization.
package scraper

import (
	"database/sql"
	"time"

	"github.com/jerryagenyi/go_ng_news_scraper/internal/config" // Fix import path
)

type ArticleScraper struct {
	db     *sql.DB
	config config.WebsiteConfig
}

func NewArticleScraper(db *sql.DB, config config.WebsiteConfig) *ArticleScraper {
	return &ArticleScraper{
		db:     db,
		config: config,
	}
}

type ArticleMetadata struct {
	Title        string
	Content      string
	Author       string
	PublishDate  time.Time
	ModifiedDate time.Time
	Categories   []int // category IDs
	URL          string
	Citations    []string
}
