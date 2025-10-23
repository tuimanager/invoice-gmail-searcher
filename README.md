# Invoice Gmail Searcher

Automatic invoice search and download from Gmail with comprehensive folder scanning.

## Features

- Smart search for invoices using configurable keywords
- All Gmail folders search with automatic folder discovery
- Encrypted storage of app passwords
- Automatic download of PDF and other attachments
- Deduplication - avoid duplicate downloads
- Service recognition: PagerDuty, Mailgun, GitHub, AWS, Google Cloud, and more
- Advanced regex patterns for payment document detection

## Installation and Usage

```bash
# Build
go build -o invoice-gmail-searcher

# First run (creates configuration)
./invoice-gmail-searcher
```

On first run, the program will:
1. Ask for your Gmail email address
2. Ask for your Gmail App Password (see setup instructions below)
3. Create a `config.json` file with default search keywords
4. Ask for the month to search (format: YYYY-MM)

For subsequent runs, the program will use the saved configuration:
```bash
# Search for specific month (program will ask for month input)
./invoice-gmail-searcher
```

## Project Structure

- `main.go` - program entry point
- `config.go` - configuration management and encryption
- `gmail.go` - Gmail IMAP connection
- `search.go` - email search and processing logic
- `attachments.go` - attachment handling

## Supported Services

Automatically recognizes invoices from:
- PagerDuty, Mailgun, GitHub, Stripe
- AWS, Google Cloud, Firebase
- Anthropic and many others

## Configuration File

After the first run, a `config.json` file is created containing:
- Your Gmail credentials (password is encrypted)
- Search keywords for invoice detection
- IMAP server settings

### Default Search Keywords

The program searches for emails containing these keywords:
- invoice, bill, receipt, payment, transaction
- charge, settlement, remittance, transfer, refund
- statement, account, balance, due, overdue
- paid, unpaid, billing, subscription, renewal

You can edit the `config.json` file to customize keywords for your needs.

## Output

- `invoices_YYYY-MM/` - folders with downloaded invoices
- Supports PDF, Excel, Word and other formats
- Excludes calendar invitations and images

## Gmail App Password Setup

### Step-by-Step Instructions:

1. **Enable 2-Factor Authentication**
   - Go to [Google Account Security](https://myaccount.google.com/security)
   - Under "Signing in to Google", enable "2-Step Verification"
   - Follow the setup process with your phone number

2. **Create App Password**
   - Return to [Google Account Security](https://myaccount.google.com/security)
   - Under "Signing in to Google", click "App passwords"
   - Select "Mail" from the app dropdown
   - Select "Other (custom name)" from device dropdown
   - Enter "Invoice Gmail Searcher" as the name
   - Click "Generate"
   - **Copy the 16-character password** (this is what you'll use, not your regular password)

3. **Use the App Password**
   - When running the program for the first time, enter your Gmail address
   - When prompted for password, enter the 16-character app password (not your regular Gmail password)
   - The password will be encrypted and stored locally in `config.json`

### Important Notes:
- Never use your regular Gmail password - only use the generated app password
- The app password is only shown once, so copy it immediately
- If you lose the app password, delete the old one and generate a new one

## Security

- Passwords are encrypted with AES-256 before saving
- Uses Gmail App Passwords for authentication
- Configuration stored locally in `config.json`