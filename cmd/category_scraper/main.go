// The category_scraper command processes news website category sitemaps.
// It extracts category information from specified Nigerian news websites
// and stores them in the database while maintaining their hierarchical
// structure. This is a prerequisite for the article scraper.
package main

import (
	"database/sql"
	"fmt"
	"log"

	"github.com/jerryagenyi/go_ng_news_scraper/internal/config"
	"github.com/jerryagenyi/go_ng_news_scraper/internal/scraper"
	_ "github.com/lib/pq"
)

func main() {
	websiteID := 1 // Blueprint.ng
	websiteConfig := config.Websites[websiteID]

	// Use the exported DBConfig instead of creating a new one
	psqlInfo := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		config.DBConfig.Host, config.DBConfig.Port, config.DBConfig.User,
		config.DBConfig.Password, config.DBConfig.DBName)

	// Initialize database connection
	db, err := sql.Open("postgres", psqlInfo)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatal(err)
	}

	categoryScraper := scraper.NewCategoryScraper(db, websiteConfig)
	if err := categoryScraper.ScrapeCategories(); err != nil {
		log.Fatal(err)
	}
}
