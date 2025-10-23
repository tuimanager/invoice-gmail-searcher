package main

import (
	"flag"
	"fmt"
	"log"
)

func main() {
	// Parse command line flags
	var month string
	var outputDir string
	flag.StringVar(&month, "month", "", "Month to search (YYYY-MM)")
	flag.StringVar(&outputDir, "output", "", "Output directory for attachments")
	flag.Parse()

	// Load configuration
	config := loadConfig()

	// Get month if not provided via flag
	if month == "" {
		month = getMonthInput()
	}
	
	if month == "" {
		log.Fatal("Month cannot be empty")
	}

	// Determine output directory
	finalOutputDir := outputDir
	if finalOutputDir == "" {
		finalOutputDir = fmt.Sprintf("invoices_%s", month)
	}

	// Connect to Gmail
	fmt.Printf("Connecting to Gmail (%s)...\n", config.Email)
	client, err := connectToGmail(config)
	if err != nil {
		log.Fatalf("Connection error: %v", err)
	}

	// Search and download attachments
	fmt.Printf("Starting search and download process...\n")
	err = searchAndDownloadAttachments(client, month, finalOutputDir, config.Email, config)
	if err != nil {
		log.Fatalf("Search error: %v", err)
	}
	
	fmt.Printf("✓ Returned from searchAndDownloadAttachments function\n")
	fmt.Printf("✓ Closing Gmail connection...\n")
	client.Close()
	fmt.Printf("✓ Gmail connection closed\n")

	fmt.Printf("Search completed. Attachments saved to: %s\n", finalOutputDir)
}