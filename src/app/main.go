package main

import (
	"crypto/aes"
	"crypto/cipher"
	"log"
	"time"
)

var config Configuration
var block cipher.Block
var logger *log.Logger

func main() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Panic recovered in main function: %v", r)
		}
	}()

	loadConfig()
	initializeLogger()
	logToFile("Starting synchronization.")

	configErr := loadConfig()
	if configErr != nil {
		log.Printf("Failed to load config: %v", configErr)
	}

	key := []byte(config.EncryptionKey) // 32 bytes
	var err error
	block, err = aes.NewCipher(key)
	if err != nil {
		handleError(err, "creating cypher")
	}

	//we run this first time only and then go over to a scheduled sync (every hour)
	runSync()

	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		runSync()
	}
}
