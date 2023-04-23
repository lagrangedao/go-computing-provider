package computing

import (
	// ... other imports ...
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func buildDockerImage(imagePath, imageName string) error {
	cmd := exec.Command("docker", "build", "-t", imageName, imagePath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		return err
	}

	return nil
}

func getSpaceName(apiURL string) (string, error) {
	parsedURL, err := url.Parse(apiURL)
	if err != nil {
		return "", err
	}

	pathSegments := parsedURL.Path[1:]
	segments := strings.Split(pathSegments, "/")

	if len(segments) < 2 || segments[0] != "spaces" {
		return "", errors.New("invalid URL format")
	}

	spaceName := segments[1]
	return spaceName, nil
}

func BuildSpaceTask(jobSourceURI string) {
	apiURL := jobSourceURI
	spaceName, err := getSpaceName(apiURL)

	log.Printf("Attempting to download spaces from Lagrange. Spaces name: %s", spaceName)

	spaceAPIURL := fmt.Sprintf(jobSourceURI)
	resp, err := http.Get(spaceAPIURL)
	if err != nil {
		log.Printf("Error making request to Space API: %v", err)
		return
	}
	defer resp.Body.Close()

	log.Printf("Space API response received. Response: %d", resp.StatusCode)

	if resp.StatusCode != http.StatusOK {
		log.Printf("Space API response not OK. Status Code: %d", resp.StatusCode)
		return
	}

	var spaceJSON struct {
		Data struct {
			Files []struct {
				Name string `json:"name"`
				URL  string `json:"url"`
			} `json:"files"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&spaceJSON); err != nil {
		log.Printf("Error decoding Space API response JSON: %v", err)
		return
	}

	files := spaceJSON.Data.Files
	buildFolder := "computing_provider/static/build/"

	if len(files) > 0 {
		downloadSpacePath := filepath.Join(filepath.Dir(files[0].Name), filepath.Base(files[0].Name))
		for _, file := range files {
			dirPath := filepath.Dir(file.Name)
			os.MkdirAll(filepath.Join(buildFolder, dirPath), os.ModePerm)

			err := downloadFile(filepath.Join(buildFolder, file.Name), file.URL)
			if err != nil {
				log.Printf("Error downloading file: %v", err)
				return
			}

			log.Printf("Download %s successfully.", spaceName)
		}

		imagePath := filepath.Join(buildFolder, filepath.Dir(downloadSpacePath))

		imageName := "lagrange/" + spaceName

		log.Printf("Image path: %s", imagePath)

		err = buildDockerImage(imagePath, imageName)
		if err != nil {
			log.Printf("Error building Docker image: %v", err)
			return
		}

		// dockerClient and other Kubernetes-related instances should be set up beforehand
		// ...
	} else {
		log.Printf("Space %s is not found.", spaceName)
	}
}
func downloadFile(filepath string, url string) error {
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}

	return nil
}
