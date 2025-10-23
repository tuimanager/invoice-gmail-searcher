package main

import (
	"bufio"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

type Config struct {
	Email             string   `json:"email"`
	Server            string   `json:"server"`
	Port              string   `json:"port"`
	EncryptedPassword string   `json:"encrypted_password"`
	Keywords          []string `json:"keywords"`
}

func loadConfig() *Config {
	configFile := "config.json"
	
	// Check if configuration file exists
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		return createNewConfig(configFile)
	}
	
	// Load existing configuration
	data, err := os.ReadFile(configFile)
	if err != nil {
		fmt.Printf("Configuration read error: %v\n", err)
		return createNewConfig(configFile)
	}
	
	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		fmt.Printf("Configuration parsing error: %v\n", err)
		return createNewConfig(configFile)
	}
	
	fmt.Printf("Using saved configuration for %s\n", config.Email)
	return &config
}

func createNewConfig(configFile string) *Config {
	config := &Config{
		Server: "imap.gmail.com",
		Port:   "993",
		Keywords: []string{
			"invoice", "bill", "receipt", "payment", "transaction",
			"charge", "settlement", "remittance", "transfer", "refund",
			"statement", "account", "balance", "due", "overdue",
			"paid", "unpaid", "billing", "subscription", "renewal",
		},
	}
	
	scanner := bufio.NewScanner(os.Stdin)
	
	fmt.Print("Email: ")
	scanner.Scan()
	config.Email = scanner.Text()
	
	fmt.Print("Gmail App Password: ")
	scanner.Scan()
	password := scanner.Text()
	
	// Encrypt password
	encryptedPassword, err := encryptPassword(password, config.Email)
	if err != nil {
		fmt.Printf("Password encryption error: %v\n", err)
		os.Exit(1)
	}
	config.EncryptedPassword = encryptedPassword
	
	// Save configuration
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		fmt.Printf("Configuration serialization error: %v\n", err)
		return config
	}
	
	err = os.WriteFile(configFile, data, 0600)
	if err != nil {
		fmt.Printf("Configuration save error: %v\n", err)
	} else {
		fmt.Printf("Configuration saved to %s\n", configFile)
	}
	
	return config
}

func encryptPassword(password, key string) (string, error) {
	// Use MD5 hash of key as AES key
	hash := md5.New()
	hash.Write([]byte(key))
	keyBytes := hash.Sum(nil)
	
	block, err := aes.NewCipher(keyBytes)
	if err != nil {
		return "", err
	}
	
	plaintext := []byte(password)
	ciphertext := make([]byte, aes.BlockSize+len(plaintext))
	iv := ciphertext[:aes.BlockSize]
	
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return "", err
	}
	
	stream := cipher.NewCFBEncrypter(block, iv)
	stream.XORKeyStream(ciphertext[aes.BlockSize:], plaintext)
	
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func decryptPassword(encryptedPassword, key string) (string, error) {
	// Use MD5 hash of key as AES key
	hash := md5.New()
	hash.Write([]byte(key))
	keyBytes := hash.Sum(nil)
	
	block, err := aes.NewCipher(keyBytes)
	if err != nil {
		return "", err
	}
	
	ciphertext, err := base64.StdEncoding.DecodeString(encryptedPassword)
	if err != nil {
		return "", err
	}
	
	if len(ciphertext) < aes.BlockSize {
		return "", fmt.Errorf("encrypted password too short")
	}
	
	iv := ciphertext[:aes.BlockSize]
	ciphertext = ciphertext[aes.BlockSize:]
	
	stream := cipher.NewCFBDecrypter(block, iv)
	stream.XORKeyStream(ciphertext, ciphertext)
	
	return string(ciphertext), nil
}

func getMonthInput() string {
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Print("Month (YYYY-MM, e.g. 2025-09): ")
	scanner.Scan()
	return strings.TrimSpace(scanner.Text())
}