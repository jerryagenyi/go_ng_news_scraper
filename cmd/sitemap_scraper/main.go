package main

import (
	"database/sql"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	_ "github.com/lib/pq"
)

// Sitemap XML structures
type URL struct {
	Loc     string `xml:"loc"`
	LastMod string `xml:"lastmod,omitempty"`
}

type URLSet struct {
	URLs []URL `xml:"url"`
}

// Config struct to hold database configuration
type Config struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
}

// Create a global config
var config = Config{
	Host:     "localhost",
	Port:     5432,
	User:     "postgres",
	Password: "naija1",
	DBName:   "ng_news",
}

// Modify these constants
const (
	maxRetries = 3
	timeout    = 90 * time.Second // Increased timeout
	retryDelay = 5 * time.Second
	batchSize  = 100 // Add batch processing
	maxWorkers = 3   // Reduce concurrent workers
)

// Add this retry function
func fetchWithRetry(client *http.Client, url string) (*http.Response, error) {
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		resp, err := client.Get(url)
		if err == nil {
			return resp, nil
		}
		lastErr = err
		log.Printf("Attempt %d failed for %s: %v. Retrying in %v...",
			i+1, url, err, retryDelay)
		time.Sleep(retryDelay)
	}
	return nil, fmt.Errorf("after %d attempts: %w", maxRetries, lastErr)
}

func main() {
	// Initialize database connection
	psqlInfo := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		config.Host, config.Port, config.User, config.Password, config.DBName)

	db, err := sql.Open("postgres", psqlInfo)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Test connection
	err = db.Ping()
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Successfully connected to database")

	// Blueprint sitemaps to process (we'll start with first 5 for testing)
	sitemaps := make([]string, 221)
	for i := 0; i < 221; i++ {
		if i == 0 {
			sitemaps[i] = "https://blueprint.ng/post-sitemap.xml"
		} else {
			sitemaps[i] = fmt.Sprintf("https://blueprint.ng/post-sitemap%d.xml", i+1)
		}
	}

	// Create a wait group to manage goroutines
	var wg sync.WaitGroup
	// Create a semaphore to limit concurrent requests
	semaphore := make(chan struct{}, maxWorkers) // Reduce from 5 to 3 workers

	// Add a counter for completed sitemaps
	var completed int32
	total := len(sitemaps)

	// Add this to main()
	ticker := time.NewTicker(30 * time.Second)
	go func() {
		for range ticker.C {
			log.Printf("Processing status: %d/%d completed", atomic.LoadInt32(&completed), total)
		}
	}()
	defer ticker.Stop()

	// Process sitemaps concurrently
	for _, sitemapURL := range sitemaps {
		wg.Add(1)
		go func(url string) {
			defer wg.Done()
			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() {
				<-semaphore
				current := atomic.AddInt32(&completed, 1)
				log.Printf("Progress: %d/%d sitemaps processed", current, total)
			}()

			if err := processSitemap(db, 1, url); err != nil {
				log.Printf("Error processing sitemap %s: %v", url, err)
			}
		}(sitemapURL)
	}

	// Wait for all goroutines to complete
	wg.Wait()
	log.Println("All sitemaps processed")
}

// Update the processSitemap function
func processSitemap(db *sql.DB, websiteID int, sitemapURL string) error {
	client := &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     30 * time.Second,
			DisableCompression:  false,
			DisableKeepAlives:   false,
		},
	}

	// Use the retry function
	resp, err := fetchWithRetry(client, sitemapURL)
	if err != nil {
		return fmt.Errorf("failed to fetch sitemap: %w", err)
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	// Parse XML
	var urlset URLSet
	err = xml.Unmarshal(body, &urlset)
	if err != nil {
		return fmt.Errorf("failed to parse XML: %w", err)
	}

	// Begin transaction
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Prepare the insert statement
	stmt, err := tx.Prepare(`
        INSERT INTO go_sitemaps (
            website_id, 
            article_url, 
            last_mod, 
            created_at, 
            is_valid, 
            status_code,
            last_checked
        )
        VALUES ($1, $2, $3, NOW(), true, $4, NOW())
        ON CONFLICT (website_id, article_url) 
        DO UPDATE SET 
            last_checked = NOW(), 
            status_code = $4, 
            is_valid = true
    `)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	// Insert URLs into database
	var batch []URL
	for _, url := range urlset.URLs {
		batch = append(batch, url)
		if len(batch) >= batchSize {
			if err := insertURLBatch(tx, batch, websiteID, resp.StatusCode); err != nil {
				log.Printf("Error inserting batch: %v", err)
			}
			batch = batch[:0]
		}
	}

	// In processSitemap function, after the batch processing loop:
	if len(batch) > 0 {
		if err := insertURLBatch(tx, batch, websiteID, resp.StatusCode); err != nil {
			log.Printf("Error inserting final batch: %v", err)
		}
	}

	// Commit transaction
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	log.Printf("Successfully processed sitemap: %s", sitemapURL)
	return nil
}

// Add the insertURLBatch function
func insertURLBatch(tx *sql.Tx, batch []URL, websiteID int, statusCode int) error {
	stmt, err := tx.Prepare(`
        INSERT INTO go_sitemaps (
            website_id, 
            article_url, 
            last_mod, 
            created_at, 
            is_valid, 
            status_code,
            last_checked
        )
        VALUES ($1, $2, $3, NOW(), true, $4, NOW())
        ON CONFLICT (website_id, article_url) 
        DO UPDATE SET 
            last_checked = NOW(), 
            status_code = $4, 
            is_valid = true
    `)
	if err != nil {
		return fmt.Errorf("failed to prepare batch statement: %w", err)
	}
	defer stmt.Close()

	for _, url := range batch {
		var lastMod *time.Time
		if url.LastMod != "" {
			parsedTime, err := time.Parse(time.RFC3339, url.LastMod)
			if err == nil {
				lastMod = &parsedTime
			}
		}

		_, err = stmt.Exec(websiteID, url.Loc, lastMod, statusCode)
		if err != nil {
			return fmt.Errorf("failed to execute batch insert: %w", err)
		}
	}
	return nil
}
