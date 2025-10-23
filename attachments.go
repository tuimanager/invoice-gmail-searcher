package main

import (
	"crypto/md5"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
)

type attachmentInfo struct {
	filename string
	section  string
}

var downloadedHashes = make(map[string]string)

var attachmentRegex = regexp.MustCompile(`(?i)` +
	`(inv(oice)?s?|bill(s|ing)?|receipt|rec|rct|cheque|check|` +
	`pay(ment)?|transaction|statement|factur[ae]|rechnung|nota)\b|` +
	`\b(INV|BILL|REC|PAY)[-_]?\d{3,}|` +
	`\d{4,}-\d{2,}-\d{2,}|` +
	`\b\d{8,}\.(pdf|xlsx?|doc[x]?|zip|png|jpe?g|gif|bmp|tiff?)\b`)

func findAttachments(bodyStructure *imap.BodyStructure, path []string) []attachmentInfo {
	var attachments []attachmentInfo
	
	if bodyStructure == nil {
		return attachments
	}
	
	currentPath := strings.Join(path, ".")
	if currentPath == "" {
		currentPath = "1"
	}
	
	// Check if this is an attachment
	if bodyStructure.Disposition == "attachment" || 
	   bodyStructure.Disposition == "inline" ||
	   (bodyStructure.DispositionParams != nil && bodyStructure.DispositionParams["filename"] != "") ||
	   (bodyStructure.Params != nil && bodyStructure.Params["name"] != "") ||
	   (bodyStructure.MIMEType == "application" && (bodyStructure.MIMESubType == "pdf" || bodyStructure.MIMESubType == "octet-stream")) ||
	   (bodyStructure.MIMEType == "application" && (bodyStructure.MIMESubType == "vnd.ms-excel" || bodyStructure.MIMESubType == "vnd.openxmlformats-officedocument.spreadsheetml.sheet")) ||
	   (bodyStructure.MIMEType == "application" && bodyStructure.MIMESubType == "zip") ||
	   (bodyStructure.MIMEType == "image" && (bodyStructure.MIMESubType == "png" || bodyStructure.MIMESubType == "jpeg" || bodyStructure.MIMESubType == "jpg" || bodyStructure.MIMESubType == "gif" || bodyStructure.MIMESubType == "bmp" || bodyStructure.MIMESubType == "tiff")) {
		
		filename := ""
		if bodyStructure.DispositionParams != nil && bodyStructure.DispositionParams["filename"] != "" {
			filename = bodyStructure.DispositionParams["filename"]
		} else if bodyStructure.Params != nil && bodyStructure.Params["name"] != "" {
			filename = bodyStructure.Params["name"]
		} else if bodyStructure.MIMEType == "application" && bodyStructure.MIMESubType == "pdf" {
			filename = "attachment.pdf"
		} else if bodyStructure.MIMEType == "application" && bodyStructure.MIMESubType == "octet-stream" {
			filename = "attachment.bin"
		} else if bodyStructure.MIMEType == "application" && bodyStructure.MIMESubType == "vnd.ms-excel" {
			filename = "attachment.xls"
		} else if bodyStructure.MIMEType == "application" && bodyStructure.MIMESubType == "vnd.openxmlformats-officedocument.spreadsheetml.sheet" {
			filename = "attachment.xlsx"
		} else if bodyStructure.MIMEType == "image" && bodyStructure.MIMESubType == "png" {
			filename = "attachment.png"
		} else if bodyStructure.MIMEType == "image" && (bodyStructure.MIMESubType == "jpeg" || bodyStructure.MIMESubType == "jpg") {
			filename = "attachment.jpg"
		} else if bodyStructure.MIMEType == "image" && bodyStructure.MIMESubType == "gif" {
			filename = "attachment.gif"
		} else if bodyStructure.MIMEType == "image" && bodyStructure.MIMESubType == "bmp" {
			filename = "attachment.bmp"
		} else if bodyStructure.MIMEType == "image" && bodyStructure.MIMESubType == "tiff" {
			filename = "attachment.tiff"
		}
		
		if filename != "" {
			attachments = append(attachments, attachmentInfo{
				filename: filename,
				section:  currentPath,
			})
		}
	}
	
	// Recursively search in nested parts
	if bodyStructure.Parts != nil {
		for i, part := range bodyStructure.Parts {
			partPath := append(path, fmt.Sprintf("%d", i+1))
			if len(path) == 0 {
				partPath = []string{fmt.Sprintf("%d", i+1)}
			}
			attachments = append(attachments, findAttachments(part, partPath)...)
		}
	}
	
	return attachments
}

func isInvoiceFile(filename string, keywords []string) bool {
	if filename == "" {
		return false
	}
	
	lower := strings.ToLower(filename)
	
	// Exclude calendar files and invitations
	if strings.Contains(lower, "invite.ics") || strings.Contains(lower, "invite") {
		return false
	}
	
	// Exclude typical interface images (logos, icons, avatars)
	excludeImages := []string{
		"logo", "icon", "avatar", "footer", "header", "banner", 
		"px48", "image-", "attach_", "signature", "spacer",
		"confluence", "atlassian", "gmail", "google",
		"screenshot", "screen shot", "screen capture",
	}
	
	for _, exclude := range excludeImages {
		if strings.Contains(lower, exclude) {
			return false
		}
	}
	
	// Check if this is an image
	isImage := strings.HasSuffix(lower, ".png") || strings.HasSuffix(lower, ".jpg") || 
			   strings.HasSuffix(lower, ".jpeg") || strings.HasSuffix(lower, ".gif") ||
			   strings.HasSuffix(lower, ".bmp") || strings.HasSuffix(lower, ".tiff")
	
	// For images, require stricter criteria
	if isImage {
		// Images must contain explicit invoice or receipt indicators from keywords
		for _, keyword := range keywords {
			if strings.Contains(lower, strings.ToLower(keyword)) {
				return true
			}
		}
		
		// Or match regex patterns
		if attachmentRegex.MatchString(filename) {
			return true
		}
		
		return false
	}
	
	// For non-images, use regular logic
	if attachmentRegex.MatchString(filename) {
		return true
	}
	
	// Special case: if generic attachment name and not image, consider potential invoice
	if !isImage && (filename == "attachment.pdf" || filename == "attachment.xlsx" || filename == "attachment.xls") {
		return true
	}
	
	// Use configured keywords for detection
	for _, keyword := range keywords {
		if strings.Contains(lower, strings.ToLower(keyword)) {
			return true
		}
	}
	
	return false
}

func detectServiceFromSubject(subject string) string {
	lower := strings.ToLower(subject)
	
	// Check more specific patterns first, then general ones
	servicePatterns := []struct {
		patterns []string
		prefix   string
	}{
		{[]string{"google workspace"}, "gworkspace"},
		{[]string{"google cloud platform", "google cloud"}, "gcloud"},
		{[]string{"digital realty"}, "digitalrealty"},
		{[]string{"trafficcake"}, "trafficcake"},
		{[]string{"statuscake"}, "statuscake"},
		{[]string{"mailgun technologies", "mailgun"}, "mailgun"},
		{[]string{"pagerduty"}, "pagerduty"},
		{[]string{"github"}, "github"},
		{[]string{"anthropic"}, "anthropic"},
		{[]string{"fastly"}, "fastly"},
		{[]string{"amazon web services", "aws"}, "aws"},
		{[]string{"stripe"}, "stripe"},
		{[]string{"firebase"}, "firebase"},
		{[]string{"twilio"}, "twilio"},
		{[]string{"slack"}, "slack"},
		{[]string{"linear orbit", "linear"}, "linear"},
		{[]string{"zoom"}, "zoom"},
		{[]string{"hetzner"}, "hetzner"},
		{[]string{"cogent communications"}, "cogent"},
		{[]string{"zayo network"}, "zayo"},
		{[]string{"lottielab"}, "lottielab"},
		{[]string{"zoominfo"}, "zoominfo"},
	}
	
	for _, service := range servicePatterns {
		for _, pattern := range service.patterns {
			if strings.Contains(lower, pattern) {
				return service.prefix
			}
		}
	}
	
	return ""
}

func detectServiceFromEmail(email string) string {
	if email == "" {
		return ""
	}
	
	lower := strings.ToLower(email)
	
	// Extract domain from email
	atIndex := strings.LastIndex(lower, "@")
	if atIndex == -1 {
		return ""
	}
	domain := lower[atIndex+1:]
	
	// Map domains to service prefixes
	domainMap := map[string]string{
		"mailgun.com": "mailgun",
		"mg.mailgun.com": "mailgun",
		"pagerduty.com": "pagerduty",
		"statuspage.pagerduty.com": "pagerduty",
		"statuscake.com": "statuscake",
		"trafficcake.com": "trafficcake",
		"github.com": "github",
		"noreply.github.com": "github",
		"anthropic.com": "anthropic",
		"workspace-noreply.google.com": "gworkspace",
		"googlecloud.com": "gcloud",
		"cloud-noreply.google.com": "gcloud",
		"google.com": "google",
		"digitalrealty.com": "digitalrealty",
		"fastly.com": "fastly",
		"amazon.com": "aws",
		"amazonaws.com": "aws",
		"awscloud.com": "aws",
		"stripe.com": "stripe",
		"firebase.google.com": "firebase",
		"twilio.com": "twilio",
		"slack.com": "slack",
		"linear.app": "linear",
		"zoom.us": "zoom",
		"hetzner.com": "hetzner",
		"cogentco.com": "cogent",
		"zayo.com": "zayo",
		"lottielab.com": "lottielab",
		"zoominfo.com": "zoominfo",
	}
	
	// Exact domain match
	if prefix, exists := domainMap[domain]; exists {
		return prefix
	}
	
	// Check subdomains (e.g. billing.stripe.com -> stripe)
	for domainKey, prefix := range domainMap {
		if strings.HasSuffix(domain, "."+domainKey) {
			return prefix
		}
	}
	
	return ""
}

func downloadAttachment(c *client.Client, uid uint32, attachment attachmentInfo, outputDir string, subject string, fromEmail string) error {
	// Check if directory exists
	if _, err := os.Stat(outputDir); os.IsNotExist(err) {
		err = os.MkdirAll(outputDir, 0755)
		if err != nil {
			return fmt.Errorf("error creating directory %s: %v", outputDir, err)
		}
	}
	
	
	// Create seqset for this specific email
	seqset := new(imap.SeqSet)
	seqset.AddNum(uid)
	
	// Request specific section
	section := &imap.BodySectionName{}
	if attachment.section != "" {
		pathStrings := strings.Split(attachment.section, ".")
		section.Path = make([]int, len(pathStrings))
		for i, p := range pathStrings {
			if p == "" {
				section.Path[i] = 1
			} else {
				var num int
				fmt.Sscanf(p, "%d", &num)
				section.Path[i] = num
			}
		}
	}
	
	messages := make(chan *imap.Message, 1)
	done := make(chan error, 1)
	go func() {
		done <- c.UidFetch(seqset, []imap.FetchItem{section.FetchItem()}, messages)
	}()
	
	var attachmentData []byte
	for msg := range messages {
		if msg.Body != nil {
			for sectionName, reader := range msg.Body {
				if sectionName.Path != nil && len(sectionName.Path) > 0 {
					data, err := io.ReadAll(reader)
					if err != nil {
						return fmt.Errorf("data read error: %v", err)
					}
					attachmentData = data
					break
				}
			}
		}
	}
	
	if err := <-done; err != nil {
		return fmt.Errorf("attachment fetch error: %v", err)
	}
	
	if len(attachmentData) == 0 {
		return fmt.Errorf("attachment is empty")
	}
	
	// Decode Base64 if necessary
	decodedData, err := base64.StdEncoding.DecodeString(string(attachmentData))
	if err != nil {
		// If decoding failed, use original data
		decodedData = attachmentData
	}
	
	// Check for duplicates by MD5 BEFORE adding numbers to filename
	hasher := md5.New()
	hasher.Write(decodedData)
	fileHash := fmt.Sprintf("%x", hasher.Sum(nil))
	
	if _, exists := downloadedHashes[fileHash]; exists {
		return nil
	}
	
	// NOW determine service and create final filename
	filename := attachment.filename
	servicePrefix := detectServiceFromEmail(fromEmail)
	if servicePrefix == "" {
		servicePrefix = detectServiceFromSubject(subject)
	}
	
	if servicePrefix != "" {
		// For generic files (attachment.*) replace completely
		if filename == "attachment.pdf" || filename == "attachment.xlsx" || filename == "attachment.xls" {
			ext := filepath.Ext(filename)
			baseName := strings.TrimSuffix(filename, ext)
			filename = fmt.Sprintf("%s_%s%s", servicePrefix, baseName, ext)
		} else {
			// For files with specific names add prefix
			ext := filepath.Ext(filename)
			baseName := strings.TrimSuffix(filename, ext)
			filename = fmt.Sprintf("%s_%s%s", servicePrefix, baseName, ext)
		}
	}
	
	// Full file path with existence check
	filePath := filepath.Join(outputDir, filename)
	
	// If file already exists, add number
	originalFilename := filename
	counter := 1
	for {
		if _, err := os.Stat(filePath); err != nil {
			// File doesn't exist, can use this name
			break
		}
		
		// File exists, create new name with number based on original name
		ext := filepath.Ext(originalFilename)
		baseName := strings.TrimSuffix(originalFilename, ext)
		numberedFilename := fmt.Sprintf("%s_%d%s", baseName, counter, ext)
		filePath = filepath.Join(outputDir, numberedFilename)
		filename = numberedFilename // Update filename for output
		counter++
		
		// Protection from infinite loop
		if counter > 100 {
			return nil
		}
	}
	
	// Save file
	err = os.WriteFile(filePath, decodedData, 0644)
	if err != nil {
		return fmt.Errorf("file save error: %v", err)
	}
	
	// Record hash
	downloadedHashes[fileHash] = filename
	
	fmt.Printf("Downloaded: %s (%d bytes)\n", filename, len(decodedData))
	return nil
}