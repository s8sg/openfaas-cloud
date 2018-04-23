package function

import (
	"archive/tar"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/openfaas/faas-cli/stack"
)

type PushEvent struct {
	Repository struct {
		Name     string `json:"name"`
		FullName string `json:"full_name"`
		CloneURL string `json:"clone_url"`
		Owner    struct {
			Login string `json:"login"`
			Email string `json:"email"`
		} `json:"owner"`
	}
	AfterCommitID string `json:"after"`
}

type tarEntry struct {
	fileName     string
	functionName string
	imageName    string
}

type cfg struct {
	Ref      string  `json:"ref"`
	Frontend *string `json:"frontend,omitempty"`
}

func parseYAML(pushEvent PushEvent, filePath string) (*stack.Services, error) {
	parsed, err := stack.ParseYAMLFile(path.Join(filePath, "stack.yml"), "", "")
	return parsed, err
}

func shrinkwrap(pushEvent PushEvent, filePath string) (string, error) {
	buildCmd := exec.Command("faas-cli", "build", "-f", "stack.yml", "--shrinkwrap")
	buildCmd.Dir = filePath
	err := buildCmd.Start()
	if err != nil {
		return "", fmt.Errorf("Cannot start faas-cli build: %t", err)
	}
	err = buildCmd.Wait()

	return filePath, err
}

func makeTar(pushEvent PushEvent, filePath string, services *stack.Services) ([]tarEntry, error) {
	tars := []tarEntry{}

	fmt.Printf("Tar up %s\n", filePath)

	for k, v := range services.Functions {
		fmt.Println("Creating tar for: ", v.Handler, k)

		tarPath := path.Join(filePath, fmt.Sprintf("%s.tar", k))
		contextTar, err := os.Create(tarPath)
		if err != nil {
			return []tarEntry{}, err
		}

		tarWriter := tar.NewWriter(contextTar)
		defer tarWriter.Close()

		base := filepath.Join(filePath, filepath.Join("build", k))

		imageName := formatImageShaTag("registry:5000", &v, pushEvent.AfterCommitID)
		config := cfg{
			Ref: imageName,
		}

		configBytes, _ := json.Marshal(config)
		configErr := ioutil.WriteFile(path.Join(base, "config"), configBytes, 0600)
		if configErr != nil {
			return nil, configErr
		}

		// fmt.Println("Base: ", base, filePath, k)
		err = filepath.Walk(base, func(path string, f os.FileInfo, pathErr error) error {
			if pathErr != nil {
				return pathErr
			}

			if f.Name() == "context.tar" {
				return nil
			}

			targetFile, err1 := os.Open(path)
			log.Println(path)

			if err1 != nil {
				return err1
			}

			header, headerErr := tar.FileInfoHeader(f, f.Name())
			if headerErr != nil {
				return headerErr
			}

			header.Name = strings.TrimPrefix(path, base)
			// log.Printf("header.Name '%s'\n", header.Name)
			if header.Name != "/config" {
				header.Name = filepath.Join("context", header.Name)
			}

			header.Name = strings.TrimPrefix(header.Name, "/")

			// log.Println("tar - header.Name ", header.Name)
			if err1 = tarWriter.WriteHeader(header); err != nil {
				return err1
			}

			if f.Mode().IsDir() {
				return nil
			}

			_, err1 = io.Copy(tarWriter, targetFile)
			return err1
		})

		if err != nil {
			return []tarEntry{}, err
		}
		tars = append(tars, tarEntry{fileName: tarPath, functionName: strings.TrimSpace(k), imageName: imageName})
	}

	return tars, nil
}

func formatImageShaTag(registry string, function *stack.Function, sha string) string {
	tag := ":latest"
	imageName := function.Image
	tagIndex := strings.LastIndex(function.Image, ":")
	if tagIndex > 0 {
		tag = function.Image[tagIndex:]
		imageName = function.Image[:tagIndex]
	}

	imageName = registry + "/" + imageName + tag + "-" + sha
	return imageName
}

func clone(pushEvent PushEvent) (string, error) {
	workDir := os.TempDir()
	destPath := path.Join(workDir, pushEvent.Repository.Name)
	if _, err := os.Stat(destPath); err == nil {
		truncateErr := os.RemoveAll(destPath)
		if truncateErr != nil {
			return "", truncateErr
		}
	}

	git := exec.Command("git", "clone", pushEvent.Repository.CloneURL)
	git.Dir = workDir
	err := git.Start()
	if err != nil {
		return "", fmt.Errorf("Cannot start git: %t", err)
	}
	err = git.Wait()

	git = exec.Command("git", "checkout", pushEvent.AfterCommitID)
	git.Dir = destPath
	err = git.Start()
	if err != nil {
		return "", fmt.Errorf("Cannot start git checkout: %t", err)
	}
	err = git.Wait()

	return destPath, err
}

func deploy(tars []tarEntry, owner string, repo string) error {

	c := http.Client{}

	for _, tarEntry := range tars {
		fmt.Println("Deploy service - " + tarEntry.functionName)

		fileOpen, err := os.Open(tarEntry.fileName)
		if err != nil {
			return err
		}

		httpReq, _ := http.NewRequest(http.MethodPost, gatewayURL+"/function/buildshiprun", fileOpen)

		httpReq.Header.Add("Repo", repo)
		httpReq.Header.Add("Owner", owner)
		httpReq.Header.Add("Service", tarEntry.functionName)
		httpReq.Header.Add("Image", tarEntry.imageName)

		res, reqErr := c.Do(httpReq)
		if reqErr != nil {
			return reqErr
		}

		fmt.Println("Service deployed ", tarEntry.functionName, res.Status, owner)
	}
	return nil
}
