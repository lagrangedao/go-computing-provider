package computing

import (
	"errors"
	"fmt"
	"github.com/filswan/go-mcs-sdk/mcs/api/common/logs"
	"github.com/lagrangedao/go-computing-provider/conf"
	"github.com/lagrangedao/go-computing-provider/docker"
	"github.com/lagrangedao/go-computing-provider/models"
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

func BuildSpaceTaskImage(spaceUuid string, files []models.SpaceFile) (bool, string, string, error) {
	var err error
	buildFolder := "build/"
	if len(files) > 0 {
		for _, file := range files {
			dirPath := filepath.Dir(file.Name)
			if err = os.MkdirAll(filepath.Join(buildFolder, dirPath), os.ModePerm); err != nil {
				return false, "", "", err
			}
			if err = downloadFile(filepath.Join(buildFolder, file.Name), file.URL); err != nil {
				return false, "", "", fmt.Errorf("error downloading file: %w", err)
			}
			logs.GetLogger().Infof("Download %s successfully.", spaceUuid)
		}

		imagePath := getDownloadPath(files[0].Name)
		var containsYaml bool
		var yamlPath string

		err = filepath.WalkDir(imagePath, func(path string, d fs.DirEntry, err error) error {
			if strings.HasSuffix(d.Name(), "deploy.yaml") || strings.HasSuffix(d.Name(), "deploy.yml") {
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
		logs.GetLogger().Warnf("Space %s is not found.", spaceUuid)
	}
	return false, "", "", NotFoundError
}

func getDownloadPath(fileName string) string {
	splits := strings.Split(fileName, "/")
	return filepath.Join(splits[0], splits[1], splits[2])
}

func BuildImagesByDockerfile(jobUuid, spaceUuid, spaceName, imagePath string) (string, string) {
	updateJobStatus(jobUuid, models.JobBuildImage)
	spaceFlag := spaceName + spaceUuid[:strings.LastIndex(spaceUuid, "-")]
	imageName := fmt.Sprintf("lagrange/%s:%d", spaceFlag, time.Now().Unix())
	if conf.GetConfig().Registry.ServerAddress != "" {
		imageName = fmt.Sprintf("%s/%s:%d",
			strings.TrimSpace(conf.GetConfig().Registry.ServerAddress), spaceFlag, time.Now().Unix())
	}
	imageName = strings.ToLower(imageName)
	dockerfilePath := filepath.Join(imagePath, "Dockerfile")
	log.Printf("Image path: %s", imagePath)

	dockerService := docker.NewDockerService()
	if err := dockerService.BuildImage(imagePath, imageName); err != nil {
		logs.GetLogger().Errorf("Error building Docker image: %v", err)
		return "", ""
	}

	if conf.GetConfig().Registry.ServerAddress != "" {
		updateJobStatus(jobUuid, models.JobPushImage)
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
