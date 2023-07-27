package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
)

func runSync() {
	logToFile("Running scheduled synchronization.")
	success, repoNames := kunnaSync()

	for _, repoName := range repoNames {
		if success {
			sendEmbedToDiscord(DiscordEmbed{
				Title:       fmt.Sprintf("Synchronization for %s Completed", repoName),
				Description: "The operation completed successfully",
				Color:       3447003,
			})
		} else {
			sendEmbedToDiscord(DiscordEmbed{
				Title:       fmt.Sprintf("Synchronization for %s Failed", repoName),
				Description: "Operation aborted due to an error, this requires immediate attention!",
				Color:       16711680,
			})
		}
	}
}

func kunnaSync() (bool, []string) {
	repoNames := make([]string, 0)
	errorMessages := make([]string, 0)
	success := true

	defer func() {
		if r := recover(); r != nil {
			logToFile(fmt.Sprintf("Panic occurred: %v", r))
			errorMessages = append(errorMessages, fmt.Sprintf("Panic occurred: %v", r))
			success = false
		}
	}()

	repos, err := fetchGitLabRepos()
	if err != nil {
		errorMessages = append(errorMessages, fmt.Sprintf("Error fetching GitLab repos: %v", err))
		return false, errorMessages
	}

	syncRepos := filterGitLabReposBySyncStatus(repos)

	if err != nil {
		errorMessages = append(errorMessages, fmt.Sprintf("Error filtering repos by sync status: %v", err))
		return false, errorMessages
	}

	for _, repo := range syncRepos {
		logToFile(fmt.Sprintf("Processing repo: %s", repo.Name))
		repoNames = append(repoNames, repo.Name)

		gitlabResult, err := getKushnResult("GitLab", repo)

		if err != nil {
			errorMessages = append(errorMessages, fmt.Sprintf("Error getting GitLab result for repo: %s, error: %s", repo.Name, err.Error()))
			continue
		}

		bunnyCDNResult, err := getKushnResult("BunnyCDN", repo)
		if err != nil {
			errorMessages = append(errorMessages, fmt.Sprintf("Error getting BunnyCDN result for repo: %s, error: %s", repo.Name, err.Error()))
			continue
		}

		if gitlabResult == nil && bunnyCDNResult == nil {
			logToFile(fmt.Sprintf("Skipping repo %s: kushn_result.json not found on both GitLab and BunnyCDN.", repo.Name))
			continue
		}

		filesToBeUploaded, filesToBeDeleted := compareFileHashes(gitlabResult, bunnyCDNResult)

		logToFile(fmt.Sprintf("Files to be uploaded: %v", filesToBeUploaded))
		logToFile(fmt.Sprintf("Files to be deleted: %v", filesToBeDeleted))

		var wg sync.WaitGroup
		sem := make(chan bool, 10)

		for _, fileHash := range filesToBeUploaded {
			wg.Add(1)
			go func(fileHash FileHash) {
				sem <- true
				logToFile("Files are being uploaded...")
				syncFile(fileHash.FileName, repo.ID, repo.Name)
				cdnOperation("PURGE", fileHash.FileName, nil, nil, repo.Name)
				os.Remove(filepath.Join(config.TempStoragePath, fileHash.FileName))
				logToFile("Files uploaded.")
				<-sem
				wg.Done()
			}(fileHash)
		}

		for _, fileHash := range filesToBeDeleted {
			if fileHash.FileName == "kushn_result.json" {
				continue
			}
			wg.Add(1)
			go func(fileHash FileHash) {
				sem <- true
				logToFile("Files are being deleted...")
				cdnOperation("DELETE", fileHash.FileName, nil, nil, repo.Name)
				cdnOperation("PURGE", fileHash.FileName, nil, nil, repo.Name)
				logToFile("Files deleted.")
				<-sem
				wg.Done()
			}(fileHash)
		}

		wg.Wait()

		secureDelete(fmt.Sprintf("%s/%s", config.TempStoragePath, repo.Name))
	}

	if len(errorMessages) > 0 {
		return false, errorMessages
	}

	return success, repoNames
}

func syncFile(fileName string, id int, repoName string) {
	if fileName == "" {
		fmt.Println("Received empty filename, skipping...")
		return
	}

	fileContent, err := cdnOperation("GITLAB_GET", fileName, nil, &id, repoName)
	if err != nil {
		handleError(err, "cdnOperation during GITLAB_GET")
	}

	encryptedContent := processFileContent(fileContent, "encrypt")
	saveToTempStorage(fileName, encryptedContent, repoName)
	decryptedContent := processFileContent(encryptedContent, "decrypt")
	cdnOperation("PUT", fileName, decryptedContent, nil, repoName)
}

func cdnOperation(mode string, fileName string, data interface{}, id *int, repoName string) ([]byte, error) {
	if mode == "GITLAB_GET" && id == nil {
		return nil, fmt.Errorf("id is required for GITLAB_GET mode")
	}

	var baseURL, requestType string
	var requestData io.Reader

	switch mode {
	case "DELETE":
		baseURL = fmt.Sprintf("%s/%s/%s/%s", config.BunnyCDNStorageUrl, config.BunnyCDNStoragePullZone, repoName, fileName)
		requestType = "DELETE"
	case "PURGE":
		baseURL = fmt.Sprintf("%s/api/purge?url=%s/%s/%s", config.BunnyCDNApiUrl, config.BunnyCDNStoragePullZone, repoName, fileName)
		requestType = "POST"
	case "GET":
		baseURL = fmt.Sprintf("%s/%s/%s", config.BunnyCDNStorageUrl, config.BunnyCDNStoragePullZone, fileName)
		requestType = "GET"
	case "PUT":
		if byteData, ok := data.([]byte); ok {
			requestData = bytes.NewBuffer(byteData)
		} else {
			return nil, fmt.Errorf("data must be of type []byte for PUT mode")
		}
		baseURL = fmt.Sprintf("%s/%s/%s/%s", config.BunnyCDNStorageUrl, config.BunnyCDNStoragePullZone, repoName, fileName)
		requestType = "PUT"
	case "GITLAB_GET":
		baseURL = fmt.Sprintf("%s/api/v4/projects/%d/repository/files/%s/raw", config.GitlabInstanceUrl, *id, fileName)
		requestType = "GET"
	default:
		return nil, fmt.Errorf("invalid mode: %s", mode)
	}

	req, err := http.NewRequest(requestType, baseURL, requestData)
	if err != nil {
		handleError(nil, fmt.Sprintf("error creating %s request: %v", requestType, err))
		return nil, fmt.Errorf("error creating %s request: %v", requestType, err)
	}

	req.Header.Add("AccessKey", config.BunnyCDNAPIKey)
	if mode == "PUT" {
		req.Header.Set("Content-Type", "application/octet-stream")
	} else if mode == "GITLAB_GET" {
		req.Header.Add("PRIVATE-TOKEN", config.GitLabAPIKey)
		req.Header.Add("Ref", "main")
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		handleError(err, "sending request")
		logToFile(err.Error())
		return nil, fmt.Errorf("error sending %s request to API: %v", requestType, err)
	}

	defer func() {
		err := resp.Body.Close()
		if err != nil {
			handleError(err, " closing response body")
		}
	}()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && fileName != "sync_config.json" && fileName != "kushn_result.json" {
		if fileName == "kushn_result.json" || fileName == "sync_config.json" {
			print("Failed to get kushn")
			return nil, err
		}
		handleError(err, fmt.Sprintf("failed to get file: %s with error code: %s", fileName, resp.Status))
		logToFile(fmt.Sprintf("error response from API on %s request: %s", requestType, resp.Status))
		return nil, fmt.Errorf("error response from API on %s request: %s", requestType, resp.Status)
	}

	if mode == "GET" || mode == "GITLAB_GET" {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			handleError(err, "reading response body")
			return nil, fmt.Errorf("error reading response body: %v", err)
		}
		return body, nil
	}

	return nil, nil
}

func fetchGitLabRepos() ([]GitLabRepo, error) {
	repos := []GitLabRepo{}

	url := fmt.Sprintf("%s/api/v4/projects", config.GitlabInstanceUrl)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		handleError(err, "creating GET Request")
	}

	req.Header.Add("PRIVATE-TOKEN", config.GitLabAPIKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		handleError(err, "Error sending request to Gitlab API")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		handleError(nil, fmt.Sprintf("error response from GitLab API on GET request: %s", resp.Status))
		return nil, fmt.Errorf("error response from GitLab API on GET request: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		handleError(err, "Reading Response Body")
	}

	if err := json.Unmarshal(body, &repos); err != nil {
		handleError(err, "umarshalling GitLab repos")
		return nil, fmt.Errorf("failed to unmarshall GitLab repos")
	}

	return repos, nil

}

func fetchSyncConfigFromRepo(repo GitLabRepo) (*SyncConfig, error) {
	configData, err := cdnOperation("GITLAB_GET", "sync_config.json", nil, &repo.ID, repo.Name)

	if err != nil {
		if err.Error() == "error response from API on GET request: 404 Not Found" {
			logToFile(fmt.Sprintf("sync_config.json not found for repo %s, skipping sync.", repo.Name))
			return nil, nil
		}
		handleError(err, "getting sync_config.json")
		return nil, err
	}

	var config SyncConfig
	if err := json.Unmarshal(configData, &config); err != nil {
		handleError(err, "unmarshalling sync_config.json")
	}

	return &config, nil
}

func filterGitLabReposBySyncStatus(repos []GitLabRepo) []GitLabRepo {
	syncRepos := []GitLabRepo{}
	for _, repo := range repos {
		syncConfig, _ := fetchSyncConfigFromRepo(repo)
		if syncConfig != nil && syncConfig.Sync {
			syncRepos = append(syncRepos, repo)
		}
	}
	return syncRepos
}

func getKushnResult(mode string, repo GitLabRepo) ([]FileHash, error) {
	var fileContent []byte
	var err error
	if mode == "GitLab" {
		fileContent, err = cdnOperation("GITLAB_GET", "kushn_result.json", nil, &repo.ID, repo.Name)
	} else if mode == "BunnyCDN" {
		fileContent, err = cdnOperation("GET", fmt.Sprintf("%s/kushn_result.json", repo.Name), nil, nil, repo.Name)
	} else {
		return nil, fmt.Errorf("invalid mode: %v. Expected 'GitLab' or 'BunnyCDN'", mode)
	}

	if err != nil {
		return nil, err
	}

	var response struct {
		Message string `json:"message"`
	}
	//GitLab doesnt seem to return a direct 404 http error, so we have to do that
	err = json.Unmarshal(fileContent, &response)
	if err == nil && response.Message == "404 File Not Found" {
		logToFile(fmt.Sprintf("kushn_result.json not found in mode: %s, treating as if sync: false is set.", mode))
		return nil, nil
	}

	var fileHashes []FileHash
	if json.Valid(fileContent) {
		err = json.Unmarshal(fileContent, &fileHashes)
		if err != nil {
			return nil, fmt.Errorf("Error unmarshalling %v kushn result: %v", mode, err)
		}
	} else {
		return nil, fmt.Errorf("invalid JSON received in %v mode", mode)
	}

	return fileHashes, nil
}
