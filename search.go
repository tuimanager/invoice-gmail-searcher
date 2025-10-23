package main

import (
	"fmt"
	"io"
	"log"
	"regexp"
	"strings"
	"time"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
)

func matchesKeywords(text string, keywords []string) bool {
	lower := strings.ToLower(text)
	for _, keyword := range keywords {
		// Use regex with word boundaries for precise matching
		pattern := `\b` + regexp.QuoteMeta(strings.ToLower(keyword)) + `\b`
		matched, err := regexp.MatchString(pattern, lower)
		if err == nil && matched {
			return true
		}
		// Fallback to simple contains check
		if strings.Contains(lower, strings.ToLower(keyword)) {
			return true
		}
	}
	return false
}

func isGroupEmail(emailAddr, userEmail string) bool {
	if emailAddr == "" || userEmail == "" {
		return false
	}
	
	// Extract domain from user email
	atIndex := strings.LastIndex(userEmail, "@")
	if atIndex == -1 {
		return false
	}
	userDomain := userEmail[atIndex+1:]
	
	// Check typical group prefixes for the same domain
	groupPrefixes := []string{"admin@", "bills@", "billing@", "dev@", "finance@", "accounting@", "ops@", "support@"}
	
	for _, prefix := range groupPrefixes {
		groupAddr := prefix + userDomain
		if strings.EqualFold(emailAddr, groupAddr) {
			return true
		}
	}
	
	return false
}

func searchAndDownloadAttachments(c *client.Client, month, outputDir, userEmail string, config *Config) error {
	// Show all available folders (removed verbose logging)
	mailboxes := make(chan *imap.MailboxInfo, 10)
	done := make(chan error, 1)
	go func() {
		done <- c.List("", "*", mailboxes)
	}()

	for range mailboxes {
		// Silent iteration
	}

	if err := <-done; err != nil {
		fmt.Printf("Error getting folder list: %v\n", err)
	}

	// Get all available folders and check potentially useful ones for invoices
	// Exclude system folders but check user folders
	allFolders := []string{}
	mailboxes2 := make(chan *imap.MailboxInfo, 50)
	done2 := make(chan error, 1)
	go func() {
		done2 <- c.List("", "*", mailboxes2)
	}()

	for m := range mailboxes2 {
		folderName := m.Name
		// Skip system folders except All Mail
		if folderName == "INBOX" || 
		   strings.HasPrefix(folderName, "[Gmail]/Drafts") ||
		   strings.HasPrefix(folderName, "[Gmail]/Sent") ||
		   strings.HasPrefix(folderName, "[Gmail]/Spam") ||
		   strings.HasPrefix(folderName, "[Gmail]/Trash") ||
		   strings.HasPrefix(folderName, "[Gmail]/Important") ||
		   strings.HasPrefix(folderName, "[Gmail]/Starred") ||
		   folderName == "[Gmail]" {
			continue
		}
		
		// Add All Mail and user folders
		if folderName == "[Gmail]/All Mail" || !strings.HasPrefix(folderName, "[Gmail]/") {
			allFolders = append(allFolders, folderName)
		}
	}

	if err := <-done2; err != nil {
		fmt.Printf("Error getting folder list for search: %v\n", err)
	}

	totalAttachments := 0
	
	for _, folderName := range allFolders {
		// Silently check folder
		_, err := c.Select(folderName, false)
		if err != nil {
			fmt.Printf("Error opening folder %s: %v\n", folderName, err)
			continue
		}
		
		// Search for invoices in folder
		specialCriteria := createSearchCriteria(month)
		specialUids, err := c.UidSearch(specialCriteria)
		if err != nil {
			fmt.Printf("Search error in %s: %v\n", folderName, err)
			continue
		}
		if len(specialUids) > 0 {
			attachmentCount := processSpecialFolder(c, specialUids, outputDir, folderName, config)
			totalAttachments += attachmentCount
		}
	}
	
	fmt.Printf("Downloaded %d attachments from additional folders\n", totalAttachments)

	// Select INBOX
	_, err := c.Select("INBOX", false)
	if err != nil {
		return err
	}

	// Removed verbose logging

	// Create search criteria for specified month
	criteria := createSearchCriteria(month)
	
	// Search emails using UidSearch
	uids, err := c.UidSearch(criteria)
	if err != nil {
		return err
	}

	// Removed verbose logging

	if len(uids) == 0 {
		return nil
	}
	
	// Process all found emails
	// Removed verbose logging

	// Process emails in batches to avoid hanging
	batchSize := 10
	inboxAttachmentCount := 0
	
	for i := 0; i < len(uids); i += batchSize {
		end := i + batchSize
		if end > len(uids) {
			end = len(uids)
		}
		
		batchUids := uids[i:end]
		// Removed verbose progress logging
		
		seqset := new(imap.SeqSet)
		seqset.AddNum(batchUids...)

		messages := make(chan *imap.Message, batchSize)
		done := make(chan error, 1)
		go func() {
			done <- c.UidFetch(seqset, []imap.FetchItem{imap.FetchEnvelope, imap.FetchBodyStructure, imap.FetchBody}, messages)
		}()

		processed := 0
		for msg := range messages {
			processed++
			
			subject := ""
			if msg.Envelope != nil && msg.Envelope.Subject != "" {
				subject = msg.Envelope.Subject
			}
			
			fromEmail := ""
			if msg.Envelope != nil && len(msg.Envelope.From) > 0 && msg.Envelope.From[0] != nil {
				fromEmail = msg.Envelope.From[0].Address()
			}

			// Removed debug To addresses logging
			
			// Removed verbose service detection logging
			
			// Check if email is invoice by subject
			isInvoiceSubject := checkInvoiceSubject(subject, config)
			
			// Determine if this is an email from group address
			isForwarded := false
			groupEmail := ""
			if msg.Envelope != nil && len(msg.Envelope.To) > 0 {
				// Check all To addresses, not just the first
				for _, to := range msg.Envelope.To {
					if to != nil {
						toAddr := to.Address()
						if isGroupEmail(toAddr, userEmail) {
							isForwarded = true
							groupEmail = toAddr
							break
						}
					}
				}
			}
			
			// Search email content for PagerDuty bank details
			containsPagerDutyBank := false
			if msg.Body != nil {
				for _, body := range msg.Body {
					if body != nil {
						bodyBytes, err := io.ReadAll(body)
						if err == nil {
							bodyText := strings.ToLower(string(bodyBytes))
							if strings.Contains(bodyText, "pagerduty invoice") || strings.Contains(bodyText, "pagerduty billing") {
								containsPagerDutyBank = true
								// Removed verbose PagerDuty detection logging
								break
							}
						}
					}
				}
			}
			
			// Find attachments
			var attachments []attachmentInfo
			if msg.BodyStructure != nil {
				attachments = findAttachments(msg.BodyStructure, []string{})
			}
			
			if len(attachments) > 0 {
				// Removed verbose attachment listing
				
				for _, attachment := range attachments {
					// Removed verbose attachment name logging
					
					isInvoiceFileName := isInvoiceFile(attachment.filename, config.Keywords)
					
					// Explicitly exclude invite files regardless of other conditions
					if strings.Contains(strings.ToLower(attachment.filename), "invite") {
						// Removed verbose skip logging
						continue
					}
					
					// Determine if this is an email from group address
					isGroupEmailMsg := isForwarded && isGroupEmail(groupEmail, userEmail)
					
					// Download if:
					// 1. File name looks like invoice, OR
					// 2. Email subject looks like invoice, OR  
					// 3. Email came to group address (admin@, bills@, dev@ etc. of same domain)
					// 4. Email contains PagerDuty bank details
					if isInvoiceFileName || isInvoiceSubject || isGroupEmailMsg || containsPagerDutyBank {
						// Removed verbose download attempt logging
						if err := downloadAttachment(c, msg.Uid, attachment, outputDir, subject, fromEmail); err != nil {
							fmt.Printf("Download error: %v\n", err)
						} else {
							inboxAttachmentCount++
							// Reason tracking removed
							fmt.Printf("Downloaded: %s\n", attachment.filename)
						}
					} else {
						// Removed verbose skip logging
					}
				}
			} else if isInvoiceSubject {
				// Removed verbose warning about emails without attachments
			}
		}
		
		if err := <-done; err != nil {
			fmt.Printf("Error processing emails: %v\n", err)
			continue
		}
		
		// Removed verbose batch completion logging
	}

	fmt.Printf("Downloaded %d attachments from INBOX\n", inboxAttachmentCount)
	fmt.Printf("Total downloaded: %d attachments (INBOX) + %d attachments (special folders) = %d attachments\n", 
		inboxAttachmentCount, totalAttachments, inboxAttachmentCount+totalAttachments)
	return nil
}

func createSearchCriteria(month string) *imap.SearchCriteria {
	if month == "" {
		log.Fatal("Month cannot be empty")
	}

	// Parse month
	date, err := time.Parse("2006-01", month)
	if err != nil {
		log.Fatal("Invalid month format (need YYYY-MM):", err)
	}

	// Check that month is not in the future
	now := time.Now()
	if date.After(now) {
		fmt.Printf("Warning: month %s is in the future, there may be no emails\n", month)
	}

	// Start and end of month
	startOfMonth := date
	endOfMonth := date.AddDate(0, 1, 0).Add(-time.Second)

	criteria := &imap.SearchCriteria{
		Since:  startOfMonth,
		Before: endOfMonth.AddDate(0, 0, 1), // Before does not include specified date
	}

	return criteria
}

func checkInvoiceSubject(subject string, config *Config) bool {
	if subject == "" {
		return false
	}
	
	// Use configured keywords for checking
	if matchesKeywords(subject, config.Keywords) {
		return true
	}
	
	// Special patterns
	lower := strings.ToLower(subject)
	
	specialWords := []string{
		"digital realty", "google workspace", "google cloud",
		"pagerduty invoice", "new pagerduty invoice",
		"mailgun", "mailgun technologies", "you have a new", "new invoice",
		"statuscake", "status cake", "trafficcake",
	}
	
	for _, word := range specialWords {
		if strings.Contains(lower, word) {
			return true
		}
	}
	
	return false
}

func truncateSubject(subject string) string {
	if len(subject) > 50 {
		return subject[:47] + "..."
	}
	return subject
}

func processSpecialFolder(c *client.Client, uids []uint32, outputDir, folderName string, config *Config) int {
	// Removed verbose folder processing logging
	
	attachmentCount := 0
	batchSize := 10
	
	for i := 0; i < len(uids); i += batchSize {
		end := i + batchSize
		if end > len(uids) {
			end = len(uids)
		}
		
		batchUids := uids[i:end]
		// Removed verbose batch progress logging
		
		seqset := new(imap.SeqSet)
		seqset.AddNum(batchUids...)

		messages := make(chan *imap.Message, batchSize)
		done := make(chan error, 1)
		go func() {
			done <- c.UidFetch(seqset, []imap.FetchItem{imap.FetchEnvelope, imap.FetchBodyStructure, imap.FetchBody}, messages)
		}()

		for msg := range messages {
			subject := ""
			if msg.Envelope != nil && msg.Envelope.Subject != "" {
				subject = msg.Envelope.Subject
			}
			
			fromEmail := ""
			if msg.Envelope != nil && len(msg.Envelope.From) > 0 && msg.Envelope.From[0] != nil {
				fromEmail = msg.Envelope.From[0].Address()
			}
			
			// Removed verbose email UID logging
			
			// Removed verbose service detection logging
			
			// Check if email subject is invoice
			isInvoiceSubject := checkInvoiceSubject(subject, config)
			if isInvoiceSubject {
				// Removed verbose subject detection logging
			}
			
			// Read content to search for PagerDuty bank details
			if msg.Body != nil {
				for _, body := range msg.Body {
					if body != nil {
						bodyBytes, err := io.ReadAll(body)
						if err == nil {
							bodyText := strings.ToLower(string(bodyBytes))
							if strings.Contains(bodyText, "pagerduty invoice") || strings.Contains(bodyText, "pagerduty billing") {
								// Removed verbose PagerDuty detection logging
								break
							}
						}
					}
				}
			}
			
			var attachments []attachmentInfo
			if msg.BodyStructure != nil {
				attachments = findAttachments(msg.BodyStructure, []string{})
			}
			
			if len(attachments) > 0 {
				// Removed verbose attachment count logging
				for _, attachment := range attachments {
					// Removed verbose attachment name logging
					
					// Explicitly exclude invite files regardless of other conditions
					if strings.Contains(strings.ToLower(attachment.filename), "invite") {
						// Removed verbose skip logging
						continue
					}
					
					// Check if this is an invoice file
					isInvoiceFileName := isInvoiceFile(attachment.filename, config.Keywords)
					
					if isInvoiceFileName {
						// Removed verbose download attempt logging
						if err := downloadAttachment(c, msg.Uid, attachment, outputDir, subject, fromEmail); err != nil {
							fmt.Printf("Download error: %v\n", err)
						} else {
							attachmentCount++
							fmt.Printf("  ✓ Downloaded: %s (from %s)\n", attachment.filename, folderName)
						}
					} else {
						// Removed verbose skip logging
					}
				}
			} else {
				// Removed verbose no-attachments logging
			}
		}
		
		if err := <-done; err != nil {
			fmt.Printf("Error processing %s emails: %v\n", folderName, err)
		}
		// Removed verbose batch completion logging
	}
	
	return attachmentCount
}