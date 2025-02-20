package main

import (
	"database/sql"
	"fmt"
	"log"

	"github.com/jerryagenyi/go_ng_news_scraper/internal/config"
	"github.com/jerryagenyi/go_ng_news_scraper/internal/scraper"
	_ "github.com/lib/pq"
)

func initDB() *sql.DB {
	psqlInfo := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		config.DBConfig.Host, config.DBConfig.Port, config.DBConfig.User,
		config.DBConfig.Password, config.DBConfig.DBName)

	db, err := sql.Open("postgres", psqlInfo)
	if err != nil {
		log.Fatal(err)
	}

	if err := db.Ping(); err != nil {
		log.Fatal(err)
	}

	log.Println("Successfully connected to database")
	return db
}

func main() {
	websiteConfig := config.Websites[1] // Blueprint.ng
	db := initDB()
	defer db.Close()

	articleScraper := scraper.NewArticleScraper(db, websiteConfig)

	// Test same article twice
	url := "https://blueprint.ng/happening-now-police-arraign-portable/"

	// First scrape
	article1, err := articleScraper.ScrapeArticle(url)
	if err != nil {
		log.Fatal(err)
	}
	hash1 := article1.ContentHash // Use the hash from the Article struct

	// Second scrape
	article2, err := articleScraper.ScrapeArticle(url)
	if err != nil {
		log.Fatal(err)
	}
	hash2 := article2.ContentHash // Use the hash from the Article struct

	fmt.Printf("Hash1: %s\n", hash1)
	fmt.Printf("Hash2: %s\n", hash2)
	fmt.Printf("Hashes match: %v\n", hash1 == hash2)
}
