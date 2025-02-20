// Package config provides configuration structures and values for the Nigerian news scraper.
// It contains database connection settings and website-specific configurations including
// sitemap locations, processing parameters, and category structure definitions.
package config

// Config holds database connection parameters.
// It provides the necessary information to establish a connection
// with the PostgreSQL database.
type Config struct {
	Host     string // Database server hostname
	Port     int    // Database server port
	User     string // Database user
	Password string // Database password
	DBName   string // Target database name
}

// Default database configuration settings.
// These values are used when no custom configuration is provided.
var DBConfig = Config{
	Host:     "localhost",
	Port:     5432,
	User:     "postgres",
	Password: "naija1",
	DBName:   "ng_news",
}

// WebsiteConfig defines the structure for website-specific settings.
// It contains all necessary parameters for scraping a specific news website,
// including sitemap locations, processing limits, and timing configurations.
type WebsiteConfig struct {
	ID                 int    // Unique identifier matching go_websites table
	Name               string // Display name of the website
	BaseURL            string // Root URL of the website
	SitemapFormat      string // Format string for sitemap URLs
	StartIndex         int    // First sitemap index
	EndIndex           int    // Last sitemap index
	MaxWorkers         int    // Maximum concurrent scraping workers
	BatchSize          int    // Number of URLs to process in one batch
	Timeout            int    // Request timeout in seconds
	RetryDelay         int    // Delay between retries in seconds
	MaxRetries         int    // Maximum number of retry attempts
	CategorySitemapURL string // URL of the category sitemap
	CategoryStructure  string // Category organization: "hierarchical" or "flat"
}

// Websites maps website IDs to their corresponding configurations.
// This map serves as the central repository of scraping configurations
// for all supported Nigerian news websites.
var Websites = map[int]WebsiteConfig{
	1: {
		ID:                 1,
		Name:               "Blue Print",
		BaseURL:            "https://blueprint.ng",
		SitemapFormat:      "https://blueprint.ng/post-sitemap%d.xml",
		StartIndex:         1,
		EndIndex:           221,
		MaxWorkers:         3,   // Optimized based on server response
		BatchSize:          100, // Balanced for performance
		Timeout:            90,  // 90 seconds to handle slow responses
		RetryDelay:         5,   // 5 seconds between retries
		MaxRetries:         3,   // Maximum 3 retry attempts
		CategorySitemapURL: "https://blueprint.ng/category-sitemap.xml",
		CategoryStructure:  "hierarchical",
	},
	// Additional websites can be added here with their specific configurations
}
