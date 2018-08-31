package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/alecthomas/kingpin"
	contracts "github.com/estafette/estafette-ci-contracts"
)

var (
	version   string
	branch    string
	revision  string
	buildDate string
	goVersion = runtime.Version()
)

var (
	// flags
	action       = kingpin.Flag("action", "Any of the following actions: build, push, tag.").Envar("ESTAFETTE_EXTENSION_ACTION").String()
	repositories = kingpin.Flag("repositories", "List of the repositories the image needs to be pushed to or tagged in.").Envar("ESTAFETTE_EXTENSION_REPOSITORIES").String()
	container    = kingpin.Flag("container", "Name of the container to build, defaults to app label if present.").Envar("ESTAFETTE_EXTENSION_CONTAINER").String()
	tags         = kingpin.Flag("tags", "List of tags the image needs to receive.").Envar("ESTAFETTE_EXTENSION_TAGS").String()
	path         = kingpin.Flag("path", "Directory to build docker container from, defaults to current working directory.").Default(".").OverrideDefaultFromEnvar("ESTAFETTE_EXTENSION_PATH").String()
	copy         = kingpin.Flag("copy", "List of files or directories to copy into the build directory.").Envar("ESTAFETTE_EXTENSION_COPY").String()
)

func main() {

	// parse command line parameters
	kingpin.Parse()

	// log to stdout and hide timestamp
	log.SetOutput(os.Stdout)
	log.SetFlags(log.Flags() &^ (log.Ldate | log.Ltime))

	// log startup message
	log.Printf("Starting estafette-extension-docker version %v...", version)

	// set defaults
	appLabel := os.Getenv("ESTAFETTE_LABEL_APP")
	if *container == "" && appLabel != "" {
		*container = appLabel
	}

	// get private container registries credentials
	credentialsJSON := os.Getenv("ESTAFETTE_CI_REPOSITORY_CREDENTIALS_JSON")
	var credentials []*contracts.ContainerRepositoryCredentialConfig
	if credentialsJSON != "" {
		json.Unmarshal([]byte(credentialsJSON), &credentials)
	}

	// validate inputs
	validateRepositories(*repositories)

	// split into arrays and set other variables
	repositoriesSlice := strings.Split(*repositories, ",")
	tagsSlice := strings.Split(*tags, ",")
	copySlice := strings.Split(*copy, ",")
	estafetteBuildVersion := os.Getenv("ESTAFETTE_BUILD_VERSION")

	switch *action {
	case "build":

		// image: extensions/docker:stable
		// action: build
		// container: docker
		// repositories:
		// - extensions
		// path: .
		// copy:
		// - Dockerfile
		// - /etc/ssl/certs/ca-certificates.crt

		// copy files/dirs from copySlice to build path
		for _, c := range copySlice {
			log.Printf("Copying %v to %v\n", c, *path)
			runCommand("cp", []string{"-r", c, *path})
		}

		// build docker image
		tagsArg := ""
		for _, r := range repositoriesSlice {
			tagsArg += fmt.Sprintf("-t %v/%v:%v", r, *container, estafetteBuildVersion)
			for _, t := range tagsSlice {
				tagsArg += fmt.Sprintf("-t %v/%v:%v", r, *container, t)
			}
		}
		args := []string{
			"build",
			tagsArg,
			*path,
		}
		runCommand("docker", args)

	case "push":

		// image: extensions/docker:stable
		// action: push
		// container: docker
		// repositories:
		// - extensions
		// tags:
		// - dev

		sourceContainerPath := fmt.Sprintf("%v/%v:%v", repositoriesSlice[0], *container, estafetteBuildVersion)

		// push each repository + tag combination
		for i, r := range repositoriesSlice {

			targetContainerPath := fmt.Sprintf("%v/%v:%v", r, *container, estafetteBuildVersion)

			if i > 0 {
				// tag container with default tag (it already exists for the first repository)
				log.Printf("Tagging container image %v\n", targetContainerPath)
				tagArgs := []string{
					"tag",
					sourceContainerPath,
					targetContainerPath,
				}
				err := exec.Command("docker", tagArgs...).Run()
				handleError(err)
			}

			loginIfRequired(credentials, targetContainerPath)

			// push container with default tag
			log.Printf("Pushing container image %v\n", targetContainerPath)
			pushArgs := []string{
				"push",
				targetContainerPath,
			}
			runCommand("docker", pushArgs)

			// push additional tags
			for _, t := range tagsSlice {

				targetContainerPath := fmt.Sprintf("%v/%v:%v", r, *container, t)

				// tag container with additional tag
				log.Printf("Tagging container image %v\n", targetContainerPath)
				tagArgs := []string{
					"tag",
					sourceContainerPath,
					targetContainerPath,
				}
				runCommand("docker", tagArgs)

				loginIfRequired(credentials, targetContainerPath)

				log.Printf("Pushing container image %v\n", targetContainerPath)
				pushArgs := []string{
					"push",
					targetContainerPath,
				}
				runCommand("docker", pushArgs)
			}
		}

	case "tag":

		// image: extensions/docker:stable
		// action: tag
		// container: docker
		// repositories:
		// - extensions
		// tags:
		// - stable
		// - latest

		sourceContainerPath := fmt.Sprintf("%v/%v:%v", repositoriesSlice[0], *container, estafetteBuildVersion)

		loginIfRequired(credentials, sourceContainerPath)

		// pull source container first
		log.Printf("Pulling container image %v\n", sourceContainerPath)
		pullArgs := []string{
			"pull",
			sourceContainerPath,
		}
		runCommand("docker", pullArgs)

		// push each repository + tag combination
		for i, r := range repositoriesSlice {

			targetContainerPath := fmt.Sprintf("%v/%v:%v", r, *container, estafetteBuildVersion)

			if i > 0 {
				// tag container with default tag
				log.Printf("Tagging container image %v\n", targetContainerPath)
				tagArgs := []string{
					"tag",
					sourceContainerPath,
					targetContainerPath,
				}
				runCommand("docker", tagArgs)

				loginIfRequired(credentials, targetContainerPath)

				// push container with default tag
				log.Printf("Pushing container image %v\n", targetContainerPath)
				pushArgs := []string{
					"push",
					targetContainerPath,
				}
				runCommand("docker", pushArgs)
			}

			// push additional tags
			for _, t := range tagsSlice {

				targetContainerPath := fmt.Sprintf("%v/%v:%v", r, *container, t)

				// tag container with additional tag
				log.Printf("Tagging container image %v\n", targetContainerPath)
				tagArgs := []string{
					"tag",
					sourceContainerPath,
					targetContainerPath,
				}
				runCommand("docker", tagArgs)

				loginIfRequired(credentials, targetContainerPath)

				log.Printf("Pushing container image %v\n", targetContainerPath)
				pushArgs := []string{
					"push",
					targetContainerPath,
				}
				runCommand("docker", pushArgs)
			}
		}

	default:
		log.Fatal("Set `command: <command>` on this step to build, push or tag")
	}
}

func validateRepositories(repositories string) {
	if repositories == "" {
		log.Fatal("Set `repositories:` to list at least one `- <repository>` (for example like `- extensions`)")
	}
}

func getCredentialsForContainer(credentials []*contracts.ContainerRepositoryCredentialConfig, containerImage string) *contracts.ContainerRepositoryCredentialConfig {
	if credentials != nil {
		for _, credentials := range credentials {
			containerImageSlice := strings.Split(containerImage, "/")
			containerRepo := strings.Join(containerImageSlice[:len(containerImageSlice)-1], "/")

			if containerRepo == credentials.Repository {
				return credentials
			}
		}
	}

	return nil
}

func loginIfRequired(credentials []*contracts.ContainerRepositoryCredentialConfig, containerImage string) {
	credential := getCredentialsForContainer(credentials, containerImage)
	if credential != nil {

		log.Printf("Logging in to repository %v for image %v\n", credential.Repository, containerImage)
		loginArgs := []string{
			"login",
			fmt.Sprintf("--username %v", credential.Username),
			fmt.Sprintf("--password %v", credential.Password),
		}

		repositorySlice := strings.Split(credential.Repository, "/")
		if len(repositorySlice) > 1 {
			server := repositorySlice[0]
			loginArgs = append(loginArgs, server)
		}

		runCommand("docker", loginArgs)
	}
}

func handleError(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func runCommand(command string, args []string) {
	cpCmd := exec.Command(command, args...)
	cpCmd.Dir = "/estafette-work"
	cpCmd.Stdout = os.Stdout
	cpCmd.Stderr = os.Stderr
	err := cpCmd.Run()
	handleError(err)
}
