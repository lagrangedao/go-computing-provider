package computing

import (
	"bufio"
	"os"

	// ... other imports ...
	"bytes"
	"fmt"
	"log"
	"os/exec"
	"regexp"
	"strings"
)

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
	cmd := exec.Command("docker", "run", "-d", "-p", "7860:7860", imageName)
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
