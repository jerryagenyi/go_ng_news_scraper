package main

import (
	"database/sql"
	"encoding/xml"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/jerryagenyi/go_ng_news_scraper/internal/config"
	"github.com/jerryagenyi/go_ng_news_scraper/internal/scraper"
	_ "github.com/lib/pq"
)

type Sitemap struct {
	XMLName xml.Name `xml:"urlset"`
	URLs    []struct {
		Loc     string `xml:"loc"`
		LastMod string `xml:"lastmod,omitempty"`
	} `xml:"url"`
}

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

	// Create a worker pool
	numWorkers := websiteConfig.MaxWorkers
	urls := make(chan string, websiteConfig.BatchSize)
	var wg sync.WaitGroup

	// Start workers
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for url := range urls {
				article, err := articleScraper.ScrapeArticle(url)
				if err != nil {
					log.Printf("Error scraping article %s: %v", url, err)
					continue
				}

				if err := articleScraper.SaveArticle(article); err != nil {
					log.Printf("Error saving article %s: %v", url, err)
				}

				// Rate limiting
				time.Sleep(time.Duration(websiteConfig.RetryDelay) * time.Second)
			}
		}()
	}

	// Get article URLs from database
	rows, err := db.Query(`
        SELECT article_url 
        FROM go_sitemaps 
        WHERE website_id = $1 
        ORDER BY created_at
    `, websiteConfig.ID)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	// Get total count of articles
	var totalArticles int
	err = db.QueryRow(`
        SELECT COUNT(*) 
        FROM go_sitemaps 
        WHERE website_id = $1
    `, websiteConfig.ID).Scan(&totalArticles)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Found %d articles to process", totalArticles)

	// Process articles directly
	processedArticles := 0
	for rows.Next() {
		processedArticles++
		var articleURL string
		if err := rows.Scan(&articleURL); err != nil {
			log.Printf("Error scanning article URL: %v", err)
			continue
		}

		// Send article URL directly to workers
		urls <- articleURL

		if processedArticles%100 == 0 {
			log.Printf("Progress: %d/%d articles queued (%.2f%%)",
				processedArticles, totalArticles,
				float64(processedArticles)/float64(totalArticles)*100)
		}
	}

	close(urls)
	wg.Wait()
	log.Println("Article scraping completed")
}
