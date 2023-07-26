package computing

import (
	// ... other imports ...
	"encoding/json"
	"errors"
	"fmt"
	"github.com/filswan/go-mcs-sdk/mcs/api/common/logs"
	"github.com/lagrangedao/go-computing-provider/conf"
	"github.com/lagrangedao/go-computing-provider/docker"
	"io"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var NotFoundError = errors.New("not found resource")

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

func BuildSpaceTaskImage(spaceName, jobSourceURI string) (bool, string, string, error) {
	logs.GetLogger().Infof("Attempting to download spaces from Lagrange. Spaces name: %s", spaceName)

	spaceAPIURL := fmt.Sprintf(jobSourceURI)
	resp, err := http.Get(spaceAPIURL)
	if err != nil {
		return false, "", "", fmt.Errorf("error making request to Space API: %w", err)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {

		}
	}(resp.Body)
	logs.GetLogger().Infof("Space API response received. Response: %d", resp.StatusCode)
	if resp.StatusCode != http.StatusOK {
		return false, "", "", fmt.Errorf("space API response not OK. Status Code: %d", resp.StatusCode)
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
		return false, "", "", fmt.Errorf("error decoding Space API response JSON: %v", err)
	}

	buildFolder := "build/"
	files := spaceJSON.Data.Files
	if len(files) > 0 {
		downloadSpacePath := filepath.Join(filepath.Dir(files[0].Name), filepath.Base(files[0].Name))
		for _, file := range files {
			dirPath := filepath.Dir(file.Name)
			if err = os.MkdirAll(filepath.Join(buildFolder, dirPath), os.ModePerm); err != nil {
				return false, "", "", err
			}
			if err = downloadFile(filepath.Join(buildFolder, file.Name), file.URL); err != nil {
				return false, "", "", fmt.Errorf("error downloading file: %w", err)
			}
			logs.GetLogger().Infof("Download %s successfully.", spaceName)
		}

		imagePath := filepath.Join(buildFolder, filepath.Dir(downloadSpacePath))
		var containsYaml bool
		var yamlPath string
		err = filepath.Walk(imagePath, func(path string, info fs.FileInfo, err error) error {
			if strings.HasSuffix(info.Name(), "deploy.yaml") || strings.HasSuffix(info.Name(), "deploy.yml") {
				containsYaml = true
				yamlPath = path
				return filepath.SkipDir
			}
			return nil
		})
		if err != nil {
			return containsYaml, yamlPath, imagePath, err
		}
		return containsYaml, yamlPath, imagePath, nil
	} else {
		logs.GetLogger().Warnf("Space %s is not found.", spaceName)
	}
	return false, "", "", NotFoundError
}

func BuildImagesByDockerfile(spaceName, imagePath string) (string, string) {
	imageName := fmt.Sprintf("lagrange/%s:%d", spaceName, time.Now().Unix())
	if conf.GetConfig().Registry.UserName != "" {
		imageName = fmt.Sprintf("%s/%s:%d",
			strings.TrimSpace(conf.GetConfig().Registry.UserName), spaceName, time.Now().Unix())
	}
	imageName = strings.ToLower(imageName)
	dockerfilePath := filepath.Join(imagePath, "Dockerfile")
	log.Printf("Image path: %s", imagePath)

	dockerService := docker.NewDockerService()
	if err := dockerService.BuildImage(imagePath, imageName); err != nil {
		logs.GetLogger().Errorf("Error building Docker image: %v", err)
		return "", ""
	}

	if conf.GetConfig().Registry.UserName != "" {
		if err := dockerService.PushImage(imageName); err != nil {
			logs.GetLogger().Errorf("Error Docker push image: %v", err)
			return "", ""
		}
	}
	return imageName, dockerfilePath
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
		return fmt.Errorf("url: %s, unexpected status code: %d", url, resp.StatusCode)
	}

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}

	return nil
}
