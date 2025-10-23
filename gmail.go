package main

import (
	"crypto/tls"
	"fmt"

	"github.com/emersion/go-imap/client"
)

func connectToGmail(config *Config) (*client.Client, error) {
	// Decrypt password
	password, err := decryptPassword(config.EncryptedPassword, config.Email)
	if err != nil {
		return nil, fmt.Errorf("password decryption error: %v", err)
	}

	address := config.Server + ":" + config.Port
	tlsConfig := &tls.Config{ServerName: config.Server}
	
	c, err := client.DialTLS(address, tlsConfig)
	if err != nil {
		return nil, err
	}

	if err := c.Login(config.Email, password); err != nil {
		c.Close()
		return nil, fmt.Errorf("authentication error: %v", err)
	}

	fmt.Println("Successfully connected to Gmail")
	return c, nil
}