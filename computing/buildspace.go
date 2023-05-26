package computing

import (
	// ... other imports ...
	"encoding/json"
	"errors"
	"fmt"
	"github.com/filswan/go-mcs-sdk/mcs/api/common/logs"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func getSpaceName(apiURL string) (string, string, error) {
	parsedURL, err := url.Parse(apiURL)
	if err != nil {
		return "", "", err
	}

	pathSegments := parsedURL.Path[1:]
	segments := strings.Split(pathSegments, "/")

	if len(segments) < 2 || segments[0] != "spaces" {
		return "", "", errors.New("invalid URL format")
	}

	creator := segments[1]
	spaceName := segments[2]
	return creator, spaceName, nil
}

func BuildSpaceTaskImage(spaceName, jobSourceURI string) (string, string) {
	log.Printf("Attempting to download spaces from Lagrange. Spaces name: %s", spaceName)

	spaceAPIURL := fmt.Sprintf(jobSourceURI)
	resp, err := http.Get(spaceAPIURL)
	if err != nil {
		log.Printf("Error making request to Space API: %v", err)
		return "", ""
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {

		}
	}(resp.Body)

	log.Printf("Space API response received. Response: %d", resp.StatusCode)

	if resp.StatusCode != http.StatusOK {
		log.Printf("Space API response not OK. Status Code: %d", resp.StatusCode)
		return "", ""
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
		return "", ""
	}

	files := spaceJSON.Data.Files
	buildFolder := "build/"

	if len(files) > 0 {
		downloadSpacePath := filepath.Join(filepath.Dir(files[0].Name), filepath.Base(files[0].Name))
		for _, file := range files {
			dirPath := filepath.Dir(file.Name)
			err := os.MkdirAll(filepath.Join(buildFolder, dirPath), os.ModePerm)
			if err != nil {
				return "", ""
			}

			err = downloadFile(filepath.Join(buildFolder, file.Name), file.URL)
			if err != nil {
				log.Printf("Error downloading file: %v", err)
				return "", ""
			}
			log.Printf("Download %s successfully.", spaceName)
		}

		imageName := fmt.Sprintf("sonic868/%s:%d", spaceName, time.Now().Unix())
		imageName = strings.ToLower(imageName)
		imagePath := filepath.Join(buildFolder, filepath.Dir(downloadSpacePath))
		dockerfilePath := filepath.Join(imagePath, "Dockerfile")
		log.Printf("Image path: %s", imagePath)

		dockerService := NewDockerService()
		if err := dockerService.BuildImage(imagePath, imageName); err != nil {
			logs.GetLogger().Errorf("Error building Docker image: %v", err)
			return "", ""
		}

		if err := dockerService.PushImage(imageName); err != nil {
			logs.GetLogger().Errorf("Error Docker push image: %v", err)
			return "", ""
		}

		return imageName, dockerfilePath
	} else {
		log.Printf("Space %s is not found.", spaceName)
	}
	return "", ""
}

func downloadFile(filepath string, url string) error {
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer func(out *os.File) {
		err := out.Close()
		if err != nil {

		}
	}(out)

	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {

		}
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}

	return nil
}

var portDomainMap = map[int]string{
	32750: "a.meta.crosschain.computer",
	32751: "b.meta.crosschain.computer",
	32752: "c.meta.crosschain.computer",
	32753: "d.meta.crosschain.computer",
	32754: "e.meta.crosschain.computer",
	32755: "f.meta.crosschain.computer",
}
