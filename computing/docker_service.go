package computing

import (
	"archive/tar"
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"go-mcs-sdk/mcs/api/common/logs"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"
)

type DockerService struct {
	c *client.Client
}

func NewDockerService() *DockerService {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		panic(err.Error())
	}
	return &DockerService{
		c: cli,
	}
}

func ExtractExposedPort(dockerfilePath string) (string, error) {
	file, err := os.Open(dockerfilePath)
	if err != nil {
		return "", fmt.Errorf("unable to open Dockerfile: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	exposedPort := ""
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(strings.TrimSpace(line), "EXPOSE") {
			re := regexp.MustCompile(`\d+`)
			exposedPort = re.FindString(line)
			break
		}
	}

	if exposedPort == "" {
		return "", fmt.Errorf("no exposed port found in Dockerfile")
	}

	return exposedPort, nil
}
func RunContainer(imageName, dockerfilePath string) string {
	exposedPort, err := ExtractExposedPort(dockerfilePath)
	if err != nil {
		log.Printf("Failed to extract exposed port: %v", err)
		return ""
	}

	portMapping := exposedPort + ":" + exposedPort
	err = RemoveContainerIfExists(imageName)
	if err != nil {
		log.Printf("Failed to remove existing container: %v", err)
		return ""
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd := exec.Command("docker", "run", "-d", "-p", portMapping, imageName)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	if err != nil {
		log.Printf("run container error: %v\n%s", err, stderr.String())
		return ""
	}

	containerID := strings.TrimSpace(stdout.String())

	// Clear the stdout buffer
	stdout.Reset()

	cmd = exec.Command("docker", "port", containerID)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	if err != nil {
		log.Printf("get container port error: %v\n%s", err, stderr.String())
		return ""
	}

	portMapping = strings.TrimSpace(stdout.String())
	fmt.Printf("Port mapping: %s\n", portMapping)

	re := regexp.MustCompile(`0\.0\.0\.0:(\d+)`)
	match := re.FindStringSubmatch(portMapping)
	if len(match) < 2 {
		log.Printf("unexpected port mapping format: %s", portMapping)
		return ""
	}

	hostPort := match[1]

	// Replace "0.0.0.0" with the desired IP address (e.g., "127.0.0.1")
	hostIP := "127.0.0.1"

	url := "http://" + hostIP + ":" + hostPort
	return url
}

func RemoveContainerIfExists(imageName string) error {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd := exec.Command("docker", "ps", "-a", "-q", "--filter", "ancestor="+imageName)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		log.Printf("list containers error: %v\n%s", err, stderr.String())
		return err
	}

	containerIDs := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(containerIDs) == 0 || containerIDs[0] == "" {
		log.Printf("No container with image %s found.", imageName)
		return nil
	}

	for _, containerID := range containerIDs {
		stdout.Reset()
		stderr.Reset()

		cmd = exec.Command("docker", "rm", "-f", containerID)
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		err = cmd.Run()
		if err != nil {
			log.Printf("remove container error: %v\n%s", err, stderr.String())
			return err
		}

		log.Printf("Removed container with ID %s", containerID)
	}

	return nil
}

func (ds *DockerService) BuildImage(buildPath, spaceName, imageName string) {
	tarPath, err := tarDir(buildPath, spaceName)
	if err != nil {
		logs.GetLogger().Errorf("Failed tar space, error: %+v", err)
		return
	}
	dockerBuildContext, err := os.Open(tarPath)
	defer dockerBuildContext.Close()

	buildResponse, err := ds.c.ImageBuild(context.Background(), dockerBuildContext, types.ImageBuildOptions{
		Context: tar.NewReader(dockerBuildContext),
		Tags:    []string{imageName},
	})
	if err != nil {
		fmt.Printf("%s", err.Error())
		return
	}
	defer buildResponse.Body.Close()
	err = printOut(buildResponse.Body)
	if err != nil {
		return
	}
}

type ErrorLine struct {
	Error       string `json:"error"`
	ErrorDetail struct {
		Message string `json:"message"`
	} `json:"errorDetail"`
}

func (ds *DockerService) PushImage(imagesName string) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*120)
	defer cancel()

	var authConfig = types.AuthConfig{
		Username:      os.Getenv("OUR_DOCKER_USERNAME"),
		Password:      os.Getenv("OUR_DOCKER_PASSWORD"),
		ServerAddress: "https://index.docker.io/v1/",
	}
	authConfigBytes, _ := json.Marshal(authConfig)
	authConfigEncoded := base64.URLEncoding.EncodeToString(authConfigBytes)

	opts := types.ImagePushOptions{RegistryAuth: authConfigEncoded}
	rd, err := ds.c.ImagePush(ctx, imagesName, opts)
	if err != nil {
		return err
	}
	defer rd.Close()

	if err = printOut(rd); err != nil {
		return err
	}
	return nil
}

func printOut(rd io.Reader) error {
	var lastLine string
	scanner := bufio.NewScanner(rd)
	for scanner.Scan() {
		lastLine = scanner.Text()
		fmt.Println(scanner.Text())
	}
	errLine := &ErrorLine{}
	json.Unmarshal([]byte(lastLine), errLine)
	if errLine.Error != "" {
		return errors.New(errLine.Error)
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return nil
}

func tarDir(buildPath, spaceName string) (string, error) {
	output := fmt.Sprintf("/tmp/build/%s.tar", spaceName)
	oldMask := syscall.Umask(0)
	defer syscall.Umask(oldMask)

	file, err := os.Create(output)
	if err != nil {
		return "", err
	}
	defer file.Close()

	tarWriter := tar.NewWriter(file)
	defer tarWriter.Close()
	filepath.Walk(buildPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		header, err := tar.FileInfoHeader(info, info.Name())
		if err != nil {
			return err
		}
		relPath, err := filepath.Rel(buildPath, path)
		if err != nil {
			return err
		}
		header.Name = relPath
		if err := tarWriter.WriteHeader(header); err != nil {
			return err
		}
		if !info.IsDir() {
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()
			if _, err := io.Copy(tarWriter, file); err != nil {
				return err
			}
		}
		return nil
	})
	fmt.Println("Archive created successfully!")
	return output, nil
}
