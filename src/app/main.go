package main

import (
	"crypto/aes"
	"crypto/cipher"
	"log"
	"os"
	"time"
)

//TODO fix "``` Inline:false}]}" being included in our console output
//TODO create a new logfile for each new sync

var config Configuration
var block cipher.Block
var logger *log.Logger
var prevLogFileName string
var logFileName string
var file *os.File

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

	runSync()

	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		initializeLogger()
		runSync()
	}
}
