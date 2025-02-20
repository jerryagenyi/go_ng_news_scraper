# Website Scraping Configurations and Observations

## Blueprint.ng (ID: 1)

- **Server Behavior**:
  - Moderate response times
  - Some timeout issues after sitemap 140
  - Handles 3 concurrent connections well
- **Optimal Settings**:
  - `maxWorkers`: 3
  - `batchSize`: 100
  - `timeout`: 90s
  - `retryDelay`: 5s
- **Notes**:
  - Total sitemaps: 221
  - Average processing time: ~30 minutes
  - Best run time: 14:10 to 14:40
  - Some timeout issues after sitemap 140

### Category Structure

- **Sitemap URL**: https://blueprint.ng/category-sitemap.xml
- **Structure Type**: Hierarchical
- **Observations**:
  - Categories have parent-child relationships
  - Some articles appear in multiple categories
  - Category pages contain additional metadata

### Academic Considerations

- **Citation Format**:
  ```
  Blueprint.ng (2024). [Article Title]. Retrieved from [URL] on [Date].
  Category: [Primary Category]/[Sub-Category]
  ```
- **Metadata Tracking**:
  - Primary and secondary categories
  - Category hierarchy preservation
  - Original publication timestamp
  - Last modification date
  - Author attribution
