package test

import (
	"archive/tar"
	"fmt"
	"go-computing-provider/computing"
	"io"
	"log"
	"os"
	"path/filepath"
	"testing"
)

func TestNewK8sService(t *testing.T) {
	service := computing.NewK8sService()
	service.GetPods("kube-system")
}

func TestGetNodeList(t *testing.T) {
	service := computing.NewK8sService()
	ip, err := service.GetNodeList()
	if err != nil {
		log.Fatalln(err)
	}
	fmt.Println(ip)
}

func TestTar(t *testing.T) {
	buildPath := "build/0xe259F84193604f9c8228940Ab5cB5c62Dfb514d6/spaces/demo001"
	spaceName := "DEMO-123"
	file, err := os.Create(fmt.Sprintf("/tmp/build/%s.tar", spaceName))
	if err != nil {
		fmt.Println(err)
		return
	}
	defer file.Close()

	tarWriter := tar.NewWriter(file)
	defer tarWriter.Close()
	filepath.Walk(buildPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Println(err)
			return err
		}
		header, err := tar.FileInfoHeader(info, info.Name())
		if err != nil {
			fmt.Println(err)
			return err
		}
		relPath, err := filepath.Rel(buildPath, path)
		if err != nil {
			fmt.Println(err)
			return err
		}
		header.Name = relPath
		if err := tarWriter.WriteHeader(header); err != nil {
			fmt.Println(err)
			return err
		}

		if !info.IsDir() {
			file, err := os.Open(path)
			if err != nil {
				fmt.Println(err)
				return err
			}
			defer file.Close()
			if _, err := io.Copy(tarWriter, file); err != nil {
				fmt.Println(err)
				return err
			}
		}
		return nil
	})

	fmt.Println("Archive created successfully!")
}

func TestDockerBuild(t *testing.T) {
	dockerService := computing.NewDockerService()
	buildPath := "build/0xe259F84193604f9c8228940Ab5cB5c62Dfb514d6/spaces/demo001"
	spaceName := "DEMO-123"
	dockerService.BuildImage(buildPath, spaceName)
}
