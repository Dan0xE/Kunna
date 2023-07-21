package main

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const discordCharLimit = 1024

func loadConfig() error {
	file, err := os.Open("config.json")
	if err != nil {
		handleError(err, "opening config file")
		return err
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	config = Configuration{}
	err = decoder.Decode(&config)
	if err != nil {
		handleError(err, "Configuration Decoding")
		return err
	}
	return nil
}

func initializeLogger() {
	if file != nil {
		err := file.Close()
		if err != nil {
			log.Printf("Failed to close the previous log file: %v", err)
		}
	}

	prevLogFileName = logFileName

	logFileName = fmt.Sprintf("log_%s.log", time.Now().Format("2006-01-02-15-04-05"))
	file, err := os.OpenFile(logFileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		handleError(err, "opening the Log file")
		return
	}

	logger = log.New(file, "", log.LstdFlags)
}

func logToFile(message string) {
	fmt.Println(message)
	logger.Print(message)
}

func handleError(err error, errLocation string) {
	log.Printf("Error at %s: %v", errLocation, err)

	if errLocation == "opening the Log file" || errLocation == "Reading Log File" {
		fmt.Println(errLocation)
		return
	}

	sendEmbedToDiscord(DiscordEmbed{
		Title:       fmt.Sprintf("Error at %s", errLocation),
		Description: fmt.Sprintf("Error details: %v", err),
		Color:       16711680,
	})
}

// TODO for now just delete the temp folder specified in the dirPath param since i have no idea how we're gonna handle this at large scale with garbage override
func secureDelete(dirPath string) error {
	err := os.RemoveAll(dirPath)
	if err != nil {
		handleError(err, "deleting dir")
		return err
	}

	return nil
}

func sendEmbedToDiscord(embed DiscordEmbed) {
	fileName := prevLogFileName
	if fileName == "" {
		if logFileName == "" {
			fmt.Println("No log file has been created yet")
			return
		}
		fileName = logFileName
	}

	logContents, err := os.ReadFile(fileName)
	if err != nil {
		handleError(err, "Reading Log File")
		return
	}

	logContentsStr := string(logContents)

	if len(logContentsStr) > discordCharLimit {
		logContentsStr = "..." + logContentsStr[len(logContentsStr)-discordCharLimit:]
	}

	logEmbedField := DiscordEmbedField{
		Name:   "Log file",
		Value:  "```" + logContentsStr + "```", // done to preserve line breaks
		Inline: false,
	}

	embed.Fields = append(embed.Fields, logEmbedField)

	body := DiscordWebhookBody{
		Content: "",
		Embeds:  []DiscordEmbed{embed},
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		handleError(err, "Creating Embed")
		return
	}

	resp, err := http.Post(config.DiscordWebHook, "application/json", bytes.NewBuffer(bodyBytes))
	if err != nil {
		handleError(err, "Posting Webhook")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		log.Printf("received status code %d from Discord\n", resp.StatusCode)
		return
	}

	log.Printf("Sent embed to Discord: %+v\n", embed)
}

// We reuse the cypher because processing the files would be super SLOW otherwise
func processFileContent(fileContent []byte, mode string) []byte {
	switch mode {
	case "encrypt":
		ciphertext := make([]byte, aes.BlockSize+len(fileContent))
		iv := ciphertext[:aes.BlockSize]
		if _, err := io.ReadFull(rand.Reader, iv); err != nil {
			logToFile(fmt.Sprintf("Error creating cipher: %v", err))
		}
		stream := cipher.NewCFBEncrypter(block, iv)
		stream.XORKeyStream(ciphertext[aes.BlockSize:], fileContent)
		return ciphertext
	case "decrypt":
		if len(fileContent) < aes.BlockSize {
			logToFile("Ciphertext too short")
		}
		iv := fileContent[:aes.BlockSize]
		fileContent = fileContent[aes.BlockSize:]
		stream := cipher.NewCFBDecrypter(block, iv)
		stream.XORKeyStream(fileContent, fileContent)
		return fileContent
	default:
		logToFile(fmt.Sprintf("Invalid mode: %v. Expected 'encrypt' or 'decrypt'", mode))
		return nil
	}
}

func saveToTempStorage(fileName string, content []byte, repoName string) string {
	if _, err := os.Stat(fmt.Sprintf("%s/%s", config.TempStoragePath, repoName)); os.IsNotExist(err) {
		err := os.MkdirAll(fmt.Sprintf("%s/%s", config.TempStoragePath, repoName), 0755) // rwxr-xr-x permissions
		if err != nil {
			handleError(err, "creating directory")
			logToFile(fmt.Sprintf("Error creating directory: %v", err))
		}
	}

	tempFilePath := filepath.Join(fmt.Sprintf("%s/%s/%s", config.TempStoragePath, repoName, fileName))
	err := os.WriteFile(tempFilePath, content, 0600) // rw------- file permissions
	if err != nil {
		handleError(err, "writing to temp file")
		logToFile(fmt.Sprintf("Error writing to temp file: %v", err))
	}
	return tempFilePath
}

func compareFileHashes(gitlabFileHashes, bunnyCDNFileHashes []FileHash) ([]FileHash, []FileHash) {
	gitlabFileHashMap := make(map[string]string)
	bunnyCDNFileHashMap := make(map[string]string)

	var filesToBeUploaded, filesToBeDeleted []FileHash

	for _, fileHash := range gitlabFileHashes {
		gitlabFileHashMap[fileHash.FileName] = fileHash.Hash
	}

	for _, fileHash := range bunnyCDNFileHashes {
		bunnyCDNFileHashMap[fileHash.FileName] = fileHash.Hash

		if _, ok := gitlabFileHashMap[fileHash.FileName]; !ok {
			filesToBeDeleted = append(filesToBeDeleted, fileHash)
		}
	}

	for _, fileHash := range gitlabFileHashes {
		if hash, ok := bunnyCDNFileHashMap[fileHash.FileName]; !ok || hash != fileHash.Hash {
			filesToBeUploaded = append(filesToBeUploaded, fileHash)
		}
	}

	logToFile(fmt.Sprintf("Files to be Uploaded: %v\n", filesToBeUploaded))
	logToFile(fmt.Sprintf("Files to be Deleted: %v\n", filesToBeDeleted))

	return filesToBeUploaded, filesToBeDeleted
}
