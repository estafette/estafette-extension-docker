package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/alecthomas/kingpin"
	foundation "github.com/estafette/estafette-foundation"
	cpy "github.com/otiai10/copy"
	"github.com/rs/zerolog/log"
)

var (
	appgroup  string
	app       string
	version   string
	branch    string
	revision  string
	buildDate string
	goVersion = runtime.Version()
)

var (
	// flags
	action                     = kingpin.Flag("action", "Any of the following actions: build, push, tag.").Envar("ESTAFETTE_EXTENSION_ACTION").String()
	repositories               = kingpin.Flag("repositories", "List of the repositories the image needs to be pushed to or tagged in.").Envar("ESTAFETTE_EXTENSION_REPOSITORIES").String()
	container                  = kingpin.Flag("container", "Name of the container to build, defaults to app label if present.").Envar("ESTAFETTE_EXTENSION_CONTAINER").String()
	tags                       = kingpin.Flag("tags", "List of tags the image needs to receive.").Envar("ESTAFETTE_EXTENSION_TAGS").String()
	path                       = kingpin.Flag("path", "Directory to build docker container from, defaults to current working directory.").Default(".").OverrideDefaultFromEnvar("ESTAFETTE_EXTENSION_PATH").String()
	dockerfile                 = kingpin.Flag("dockerfile", "Dockerfile to build, defaults to Dockerfile.").Default("Dockerfile").OverrideDefaultFromEnvar("ESTAFETTE_EXTENSION_DOCKERFILE").String()
	inlineDockerfile           = kingpin.Flag("inline", "Dockerfile to build inlined.").Envar("ESTAFETTE_EXTENSION_INLINE").String()
	copy                       = kingpin.Flag("copy", "List of files or directories to copy into the build directory.").Envar("ESTAFETTE_EXTENSION_COPY").String()
	target                     = kingpin.Flag("target", "Specify which stage to target for multi-stage build.").Envar("ESTAFETTE_EXTENSION_TARGET").String()
	args                       = kingpin.Flag("args", "List of build arguments to pass to the build.").Envar("ESTAFETTE_EXTENSION_ARGS").String()
	pushVersionTag             = kingpin.Flag("push-version-tag", "By default the version tag is pushed, so it can be promoted with a release, but if you don't want it you can disable it via this flag.").Default("true").Envar("ESTAFETTE_EXTENSION_PUSH_VERSION_TAG").Bool()
	versionTagPrefix           = kingpin.Flag("version-tag-prefix", "A prefix to add to the version tag so promoting different containers originating from the same pipeline is possible.").Envar("ESTAFETTE_EXTENSION_VERSION_TAG_PREFIX").String()
	versionTagSuffix           = kingpin.Flag("version-tag-suffix", "A suffix to add to the version tag so promoting different containers originating from the same pipeline is possible.").Envar("ESTAFETTE_EXTENSION_VERSION_TAG_SUFFIX").String()
	noCache                    = kingpin.Flag("no-cache", "Indicates cache shouldn't be used when building the image.").Default("false").Envar("ESTAFETTE_EXTENSION_NO_CACHE").Bool()
	expandEnvironmentVariables = kingpin.Flag("expand-envvars", "By default environment variables get replaced in the Dockerfile, use this flag to disable that behaviour").Default("true").Envar("ESTAFETTE_EXTENSION_EXPAND_VARIABLES").Bool()
	dontExpand                 = kingpin.Flag("dont-expand", "Comma separate list of environment variable names that should not be expanded").Default("PATH").Envar("ESTAFETTE_EXTENSION_DONT_EXPAND").String()

	gitSource = kingpin.Flag("git-source", "Repository source.").Envar("ESTAFETTE_GIT_SOURCE").String()
	gitOwner  = kingpin.Flag("git-owner", "Repository owner.").Envar("ESTAFETTE_GIT_OWNER").String()
	gitName   = kingpin.Flag("git-name", "Repository name, used as application name if not passed explicitly and app label not being set.").Envar("ESTAFETTE_GIT_NAME").String()
	gitBranch = kingpin.Flag("git-branch", "Git branch to tag image with for improved caching.").Envar("ESTAFETTE_GIT_BRANCH").String()
	appLabel  = kingpin.Flag("app-name", "App label, used as application name if not passed explicitly.").Envar("ESTAFETTE_LABEL_APP").String()

	minimumSeverityToFail = kingpin.Flag("minimum-severity-to-fail", "Minimum severity of detected vulnerabilities to fail the build on").Default("CRITICAL").Envar("ESTAFETTE_EXTENSION_SEVERITY").String()
	saveContainerForTrivy = kingpin.Flag("save-container-for-trivy", "When enabled the docker image is written to disk before running trivy against is.").Default("false").Envar("ESTAFETTE_EXTENSION_SAVE_CONTAINER_FOR_TRIVY").Bool()

	credentialsJSON    = kingpin.Flag("credentials", "Container registry credentials configured at the CI server, passed in to this trusted extension.").Envar("ESTAFETTE_CREDENTIALS_CONTAINER_REGISTRY").String()
	githubAPITokenJSON = kingpin.Flag("githubApiToken", "Github api token credentials configured at the CI server, passed in to this trusted extension.").Envar("ESTAFETTE_CREDENTIALS_GITHUB_API_TOKEN").String()
)

func main() {

	// parse command line parameters
	kingpin.Parse()

	// init log format from envvar ESTAFETTE_LOG_FORMAT
	foundation.InitLoggingFromEnv(appgroup, app, version, branch, revision, buildDate)

	// create context to cancel commands on sigterm
	ctx := foundation.InitCancellationContext(context.Background())

	if runtime.GOOS == "windows" {
		interfaces, err := net.Interfaces()
		if err != nil {
			log.Print(err)
		} else {
			log.Info().Msgf("Listing network interfaces and their MTU: %v", interfaces)
		}
	}

	// set defaults
	if *container == "" && *appLabel == "" && *gitName != "" {
		*container = *gitName
	}
	if *container == "" && *appLabel != "" {
		*container = *appLabel
	}

	// get api token from injected credentials
	var credentials []ContainerRegistryCredentials
	if *credentialsJSON != "" {
		err := json.Unmarshal([]byte(*credentialsJSON), &credentials)
		if err != nil {
			log.Fatal().Err(err).Msg("Failed unmarshalling injected credentials")
		}
	}

	if *githubAPITokenJSON != "" {
		var githubAPIToken []APITokenCredentials
		err := json.Unmarshal([]byte(*githubAPITokenJSON), &githubAPIToken)

		if err != nil {
			log.Fatal().Err(err).Msg("Failed unmarshalling injected github api token credentials")
		}
		if len(githubAPIToken) > 0 {
			// set as env, so it gets used by Trivy to avoid github api rate limits when downloading db
			os.Setenv("GITHUB_TOKEN", githubAPIToken[0].AdditionalProperties.Token)
		}
	}

	// validate inputs
	validateRepositories(*repositories)

	// split into arrays and set other variables
	var repositoriesSlice []string
	if *repositories != "" {
		repositoriesSlice = strings.Split(*repositories, ",")
	}
	var tagsSlice []string
	if *tags != "" {
		tagsSlice = strings.Split(*tags, ",")
	}
	var copySlice []string
	if *copy != "" {
		copySlice = strings.Split(*copy, ",")
	}
	var argsSlice []string
	if *args != "" {
		argsSlice = strings.Split(*args, ",")
	}
	estafetteBuildVersion := os.Getenv("ESTAFETTE_BUILD_VERSION")
	estafetteBuildVersionAsTag := tidyTag(estafetteBuildVersion)
	if *versionTagPrefix != "" {
		estafetteBuildVersionAsTag = tidyTag(*versionTagPrefix + "-" + estafetteBuildVersionAsTag)
	}
	if *versionTagSuffix != "" {
		estafetteBuildVersionAsTag = tidyTag(estafetteBuildVersionAsTag + "-" + *versionTagSuffix)
	}
	gitBranchAsTag := tidyTag(fmt.Sprintf("cache-%v", *gitBranch))

	switch *action {
	case "build":

		// minimal using defaults

		// image: extensions/docker:stable
		// action: build
		// repositories:
		// - extensions

		// with defaults:

		// path: .
		// container: ${ESTAFETTE_GIT_NAME}
		// dockerfile: Dockerfile

		// or use a more verbose version to override defaults

		// image: extensions/docker:stable
		// env: SOME_BUILD_ARG_ENVVAR
		// action: build
		// container: docker
		// dockerfile: Dockerfile
		// repositories:
		// - extensions
		// path: .
		// copy:
		// - Dockerfile
		// - /etc/ssl/certs/ca-certificates.crt
		// args:
		// - SOME_BUILD_ARG_ENVVAR

		// make build dir if it doesn't exist
		log.Info().Msgf("Ensuring build directory %v exists", *path)
		if ok, _ := pathExists(*path); !ok {
			err := os.MkdirAll(*path, os.ModePerm)
			foundation.HandleError(err)
		}

		// copy files/dirs from copySlice to build path
		for _, c := range copySlice {

			fi, err := os.Stat(c)
			foundation.HandleError(err)
			switch mode := fi.Mode(); {
			case mode.IsDir():
				log.Info().Msgf("Copying directory %v to %v", c, *path)
				err := cpy.Copy(c, filepath.Join(*path, filepath.Base(c)))
				foundation.HandleError(err)

			case mode.IsRegular():
				log.Info().Msgf("Copying file %v to %v", c, *path)
				err := cpy.Copy(c, filepath.Join(*path, filepath.Base(c)))
				foundation.HandleError(err)

			default:
				log.Fatal().Msgf("Unknown file mode %v for path %v", mode, c)
			}
		}

		sourceDockerfilePath := ""
		targetDockerfilePath := filepath.Join(*path, filepath.Base(*dockerfile))
		sourceDockerfile := ""

		// check in order of importance whether `inline` dockerfile is set, path to `dockerfile` is set or a dockerfile exist in /template directory (for building docker extension from this one)
		if *inlineDockerfile != "" {
			sourceDockerfile = *inlineDockerfile
		} else if _, err := os.Stat(*dockerfile); !os.IsNotExist(err) {
			sourceDockerfilePath = *dockerfile
		} else if _, err := os.Stat("/template/Dockerfile"); !os.IsNotExist(err) {
			sourceDockerfilePath = "/template/Dockerfile"
		} else {
			log.Fatal().Msg("No Dockerfile can be found; either use the `inline` property, set the path to a Dockerfile with the `dockerfile` property or inherit from the Docker extension and store a Dockerfile at /template/Dockerfile")
		}

		if sourceDockerfile == "" && sourceDockerfilePath != "" {
			log.Info().Msgf("Reading dockerfile content from %v...", sourceDockerfilePath)
			data, err := ioutil.ReadFile(sourceDockerfilePath)
			foundation.HandleError(err)
			sourceDockerfile = string(data)
		}

		targetDockerfile := sourceDockerfile
		if *expandEnvironmentVariables {
			log.Print("Expanding environment variables in Dockerfile...")
			targetDockerfile = expandEnvironmentVariablesIfSet(sourceDockerfile, dontExpand)
		}

		log.Info().Msgf("Writing Dockerfile to %v...", targetDockerfilePath)
		err := ioutil.WriteFile(targetDockerfilePath, []byte(targetDockerfile), 0644)
		foundation.HandleError(err)

		// list directory content
		log.Info().Msgf("Listing directory %v content", *path)
		files, err := ioutil.ReadDir(*path)
		foundation.HandleError(err)
		for _, f := range files {
			if f.IsDir() {
				log.Info().Msgf("- %v/", f.Name())
			} else {
				log.Info().Msgf("- %v", f.Name())
			}
		}

		// find all images in FROM statements in dockerfile
		fromImagePaths, err := getFromImagePathsFromDockerfile(targetDockerfile)
		foundation.HandleError(err)

		// pull images in advance so we can log in to different repositories in the same registry (see https://github.com/moby/moby/issues/37569)
		for _, i := range fromImagePaths {
			loginIfRequired(credentials, false, i)
			log.Info().Msgf("Pulling container image %v", i)
			pullArgs := []string{
				"pull",
				i,
			}
			foundation.RunCommandWithArgs(ctx, "docker", pullArgs)
		}

		// login to registry for destination container image
		containerPath := fmt.Sprintf("%v/%v:%v", repositoriesSlice[0], *container, estafetteBuildVersionAsTag)
		loginIfRequired(credentials, false, containerPath)
		cacheContainerPath := fmt.Sprintf("%v/%v:%v", repositoriesSlice[0], *container, gitBranchAsTag)

		if !*noCache {
			log.Info().Msgf("Pulling docker image %v to use as cache during build...", cacheContainerPath)
			pullArgs := []string{
				"pull",
				cacheContainerPath,
			}
			// ignore if it fails
			foundation.RunCommandWithArgsExtended(ctx, "docker", pullArgs)
		}

		// build docker image
		log.Info().Msgf("Building docker image %v...", containerPath)

		log.Info().Msg("")
		fmt.Println(targetDockerfile)
		log.Info().Msg("")

		args := []string{
			"build",
		}
		if *noCache {
			args = append(args, "--no-cache")
		}
		args = append(args, "--tag", cacheContainerPath)
		for _, r := range repositoriesSlice {
			args = append(args, "--tag", fmt.Sprintf("%v/%v:%v", r, *container, estafetteBuildVersionAsTag))
			for _, t := range tagsSlice {
				if r == repositoriesSlice[0] && (t == estafetteBuildVersionAsTag || t == gitBranchAsTag) {
					continue
				}
				args = append(args, "--tag", fmt.Sprintf("%v/%v:%v", r, *container, t))
			}
		}
		if *target != "" {
			args = append(args, "--target", *target)
		}
		for _, a := range argsSlice {
			argValue := os.Getenv(a)
			args = append(args, "--build-arg", fmt.Sprintf("%v=%v", a, argValue))
		}

		if !*noCache {
			args = append(args, "--cache-from", cacheContainerPath)
		}
		args = append(args, "--file", targetDockerfilePath)
		args = append(args, *path)
		foundation.RunCommandWithArgs(ctx, "docker", args)

		// run trivy for CRITICAL
		severityArgument := "UNKNOWN,LOW,MEDIUM,HIGH,CRITICAL"
		switch strings.ToUpper(*minimumSeverityToFail) {
		case "UNKNOWN":
			severityArgument = "UNKNOWN,LOW,MEDIUM,HIGH,CRITICAL"
		case "LOW":
			severityArgument = "LOW,MEDIUM,HIGH,CRITICAL"
		case "MEDIUM":
			severityArgument = "MEDIUM,HIGH,CRITICAL"
		case "HIGH":
			severityArgument = "HIGH,CRITICAL"
		case "CRITICAL":
			severityArgument = "CRITICAL"
		}

		// update trivy db, ignore errors
		log.Info().Msg("Updating trivy vulnerabilities database...")
		_ = foundation.RunCommandWithArgsExtended(ctx, "/trivy", []string{"--cache-dir", "/trivy-cache", "image", "--light", "--download-db-only", containerPath})

		if *saveContainerForTrivy {
			log.Info().Msg("Saving docker image to file for scanning...")
			tmpfile, err := ioutil.TempFile("", "*.tar")
			if err != nil {
				log.Fatal().Err(err).Msg("Failed creating temporary file")
			}
			foundation.RunCommandWithArgs(ctx, "docker", []string{"save", containerPath, "-o", tmpfile.Name()})

			log.Info().Msgf("Scanning container image %v for vulnerabilities of severities %v...", containerPath, severityArgument)
			err = foundation.RunCommandWithArgsExtended(ctx, "/trivy", []string{"--cache-dir", "/trivy-cache", "image", "--severity", severityArgument, "--light", "--skip-update", "--no-progress", "--exit-code", "15", "--ignore-unfixed", "--input", tmpfile.Name()})
		} else {
			log.Info().Msgf("Scanning container image %v for vulnerabilities of severities %v...", containerPath, severityArgument)
			err = foundation.RunCommandWithArgsExtended(ctx, "/trivy", []string{"--cache-dir", "/trivy-cache", "image", "--severity", severityArgument, "--light", "--skip-update", "--no-progress", "--exit-code", "15", "--ignore-unfixed", containerPath})
		}

		if err != nil {
			if strings.EqualFold(err.Error(), "exit status 1") {
				// ignore exit code, until trivy fixes this on their side, see https://github.com/aquasecurity/trivy/issues/8
				// await https://github.com/aquasecurity/trivy/pull/476 to be released
				log.Warn().Msg("Ignoring Unknown OS error")
			} else {
				log.Fatal().Err(err).Msgf("The container image has vulnerabilities of severity %v! Look at https://estafette.io/security/vulnerabilities/ to learn how to fix vulnerabilities in your image.", severityArgument)
			}
		}

	case "push":

		// image: extensions/docker:stable
		// action: push
		// container: docker
		// repositories:
		// - extensions
		// tags:
		// - dev

		sourceContainerPath := fmt.Sprintf("%v/%v:%v", repositoriesSlice[0], *container, estafetteBuildVersionAsTag)
		cacheContainerPath := fmt.Sprintf("%v/%v:%v", repositoriesSlice[0], *container, gitBranchAsTag)

		// push each repository + tag combination
		for i, r := range repositoriesSlice {

			targetContainerPath := fmt.Sprintf("%v/%v:%v", r, *container, estafetteBuildVersionAsTag)

			if i > 0 {
				// tag container with default tag (it already exists for the first repository)
				log.Info().Msgf("Tagging container image %v", targetContainerPath)
				tagArgs := []string{
					"tag",
					sourceContainerPath,
					targetContainerPath,
				}
				err := exec.Command("docker", tagArgs...).Run()
				foundation.HandleError(err)
			}

			loginIfRequired(credentials, true, targetContainerPath)

			if *pushVersionTag {
				// push container with default tag
				log.Info().Msgf("Pushing container image %v", targetContainerPath)
				pushArgs := []string{
					"push",
					targetContainerPath,
				}
				foundation.RunCommandWithArgs(ctx, "docker", pushArgs)
			} else {
				log.Info().Msg("Skipping pushing version tag, because pushVersionTag is set to false; this make promoting a version to a tag at a later stage impossible!")
			}

			if !*pushVersionTag && len(tagsSlice) == 0 {
				log.Fatal().Msg("When setting pushVersionTag to false you need at least one tag")
			}

			if !*noCache {
				log.Info().Msgf("Pushing cache container image %v", cacheContainerPath)
				pushArgs := []string{
					"push",
					cacheContainerPath,
				}
				foundation.RunCommandWithArgs(ctx, "docker", pushArgs)
			}

			// push additional tags
			for _, t := range tagsSlice {

				if r == repositoriesSlice[0] && (t == estafetteBuildVersionAsTag || t == gitBranchAsTag) {
					continue
				}

				targetContainerPath := fmt.Sprintf("%v/%v:%v", r, *container, t)

				// tag container with additional tag
				log.Info().Msgf("Tagging container image %v", targetContainerPath)
				tagArgs := []string{
					"tag",
					sourceContainerPath,
					targetContainerPath,
				}
				foundation.RunCommandWithArgs(ctx, "docker", tagArgs)

				loginIfRequired(credentials, true, targetContainerPath)

				log.Info().Msgf("Pushing container image %v", targetContainerPath)
				pushArgs := []string{
					"push",
					targetContainerPath,
				}
				foundation.RunCommandWithArgs(ctx, "docker", pushArgs)
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

		sourceContainerPath := fmt.Sprintf("%v/%v:%v", repositoriesSlice[0], *container, estafetteBuildVersionAsTag)

		loginIfRequired(credentials, false, sourceContainerPath)

		// pull source container first
		log.Info().Msgf("Pulling container image %v", sourceContainerPath)
		pullArgs := []string{
			"pull",
			sourceContainerPath,
		}
		foundation.RunCommandWithArgs(ctx, "docker", pullArgs)

		// push each repository + tag combination
		for i, r := range repositoriesSlice {

			targetContainerPath := fmt.Sprintf("%v/%v:%v", r, *container, estafetteBuildVersionAsTag)

			if i > 0 {
				// tag container with default tag
				log.Info().Msgf("Tagging container image %v", targetContainerPath)
				tagArgs := []string{
					"tag",
					sourceContainerPath,
					targetContainerPath,
				}
				foundation.RunCommandWithArgs(ctx, "docker", tagArgs)

				loginIfRequired(credentials, true, targetContainerPath)

				// push container with default tag
				log.Info().Msgf("Pushing container image %v", targetContainerPath)
				pushArgs := []string{
					"push",
					targetContainerPath,
				}
				foundation.RunCommandWithArgs(ctx, "docker", pushArgs)
			}

			// push additional tags
			for _, t := range tagsSlice {

				targetContainerPath := fmt.Sprintf("%v/%v:%v", r, *container, t)

				// tag container with additional tag
				log.Info().Msgf("Tagging container image %v", targetContainerPath)
				tagArgs := []string{
					"tag",
					sourceContainerPath,
					targetContainerPath,
				}
				foundation.RunCommandWithArgs(ctx, "docker", tagArgs)

				loginIfRequired(credentials, true, targetContainerPath)

				log.Info().Msgf("Pushing container image %v", targetContainerPath)
				pushArgs := []string{
					"push",
					targetContainerPath,
				}
				foundation.RunCommandWithArgs(ctx, "docker", pushArgs)
			}
		}

	case "dive":

		containerPath := fmt.Sprintf("%v/%v:%v", repositoriesSlice[0], *container, estafetteBuildVersionAsTag)

		log.Info().Msgf("Inspecting container image %v layers...", containerPath)
		os.Setenv("CI", "true")
		foundation.RunCommandWithArgs(ctx, "/dive", []string{containerPath})

	case "trivy":

		containerPath := fmt.Sprintf("%v/%v:%v", repositoriesSlice[0], *container, estafetteBuildVersionAsTag)

		// update trivy db, ignore errors
		log.Info().Msg("Updating trivy vulnerabilities database...")
		_ = foundation.RunCommandWithArgsExtended(ctx, "/trivy", []string{"--cache-dir", "/trivy-cache", "image", "--light", "--download-db-only", containerPath})

		var err error
		if *saveContainerForTrivy {
			log.Info().Msg("Saving docker image to file for scanning...")
			tmpfile, err := ioutil.TempFile("", "*.tar")
			if err != nil {
				log.Fatal().Err(err).Msg("Failed creating temporary file")
			}
			foundation.RunCommandWithArgs(ctx, "docker", []string{"save", containerPath, "-o", tmpfile.Name()})

			log.Info().Msgf("Scanning container image %v for vulnerabilities...", containerPath)
			err = foundation.RunCommandWithArgsExtended(ctx, "/trivy", []string{"--cache-dir", "/trivy-cache", "image", "--light", "--skip-update", "--no-progress", "--exit-code", "15", "--ignore-unfixed", "--input", tmpfile.Name()})
		} else {
			log.Info().Msgf("Scanning container image %v for vulnerabilities...", containerPath)
			err = foundation.RunCommandWithArgsExtended(ctx, "/trivy", []string{"--cache-dir", "/trivy-cache", "image", "--light", "--skip-update", "--no-progress", "--exit-code", "15", "--ignore-unfixed", containerPath})
		}

		if err != nil {
			if strings.EqualFold(err.Error(), "exit status 1") {
				// ignore exit code, until trivy fixes this on their side, see https://github.com/aquasecurity/trivy/issues/8
				// await https://github.com/aquasecurity/trivy/pull/476 to be released
				log.Warn().Msg("Ignoring Unknown OS error")
			} else {
				log.Fatal().Err(err).Msgf("The container image has vulnerabilities! Look at https://estafette.io/security/vulnerabilities/ to learn how to fix vulnerabilities in your image.")
			}
		}

	default:
		log.Fatal().Msg("Set `command: <command>` on this step to build, push or tag")
	}
}

func validateRepositories(repositories string) {
	if repositories == "" {
		log.Fatal().Msg("Set `repositories:` to list at least one `- <repository>` (for example like `- extensions`)")
	}
}

func getCredentialsForContainers(credentials []ContainerRegistryCredentials, containerImages []string) map[string]*ContainerRegistryCredentials {

	filteredCredentialsMap := make(map[string]*ContainerRegistryCredentials, 0)

	if credentials != nil {
		// loop all container images
		for _, ci := range containerImages {
			containerImageSlice := strings.Split(ci, "/")
			containerRepo := strings.Join(containerImageSlice[:len(containerImageSlice)-1], "/")

			if _, ok := filteredCredentialsMap[containerRepo]; ok {
				// credentials for this repo were added before, check next container image
				continue
			}

			// find the credentials matching the container image
			for _, credential := range credentials {
				if containerRepo == credential.AdditionalProperties.Repository {

					// this one matches, add it to the map
					filteredCredentialsMap[credential.AdditionalProperties.Repository] = &credential
					break
				}
			}
		}
	}

	return filteredCredentialsMap
}

// isAllowedPipelineForPush returns true if allowedPipelinesToPush is empty or matches the pipelines full path
func isAllowedPipelineForPush(credential ContainerRegistryCredentials, fullRepositoryPath string) bool {

	if credential.AdditionalProperties.AllowedPipelinesToPush == "" {
		return true
	}

	pattern := fmt.Sprintf("^%v$", strings.TrimSpace(credential.AdditionalProperties.AllowedPipelinesToPush))
	isMatch, _ := regexp.Match(pattern, []byte(fullRepositoryPath))

	return isMatch
}

var (
	imagesFromDockerFileRegex *regexp.Regexp
)

func getFromImagePathsFromDockerfile(dockerfileContent string) ([]string, error) {

	containerImages := []string{}

	if imagesFromDockerFileRegex == nil {
		imagesFromDockerFileRegex = regexp.MustCompile(`(?im)^FROM\s*([^\s]+)(\s*AS\s[a-zA-Z0-9]+)?\s*$`)
	}

	matches := imagesFromDockerFileRegex.FindAllStringSubmatch(dockerfileContent, -1)

	if len(matches) > 0 {
		for _, m := range matches {
			if len(m) > 1 {
				// check if it's not an official docker hub image
				if strings.Count(m[1], "/") != 0 && !strings.Contains(m[1], "$") {
					containerImages = append(containerImages, m[1])
				}
			}
		}
	}

	return containerImages, nil
}

func loginIfRequired(credentials []ContainerRegistryCredentials, push bool, containerImages ...string) {

	log.Info().Msgf("Filtering credentials for images %v", containerImages)

	// retrieve all credentials
	filteredCredentialsMap := getCredentialsForContainers(credentials, containerImages)

	log.Info().Msgf("Filtered %v container-registry credentials down to %v", len(credentials), len(filteredCredentialsMap))

	if filteredCredentialsMap != nil {
		for _, c := range filteredCredentialsMap {
			if c != nil {

				fullRepositoryPath := fmt.Sprintf("%v/%v/%v", *gitSource, *gitOwner, *gitName)
				if push && !isAllowedPipelineForPush(*c, fullRepositoryPath) {
					log.Info().Msgf("Pushing to repository '%v' is not allowed, skipping login", c.AdditionalProperties.Repository)
					continue
				}

				log.Info().Msgf("Logging in to repository '%v'", c.AdditionalProperties.Repository)
				loginArgs := []string{
					"login",
					"--username",
					c.AdditionalProperties.Username,
					"--password",
					c.AdditionalProperties.Password,
				}

				repositorySlice := strings.Split(c.AdditionalProperties.Repository, "/")
				if len(repositorySlice) > 1 {
					server := repositorySlice[0]
					loginArgs = append(loginArgs, server)
				}

				err := exec.Command("docker", loginArgs...).Run()
				foundation.HandleError(err)
			}
		}
	}
}

func tidyTag(tag string) string {
	// A tag name must be valid ASCII and may contain lowercase and uppercase letters, digits, underscores, periods and dashes.
	tag = regexp.MustCompile(`[^a-zA-Z0-9_.\-]+`).ReplaceAllString(tag, "-")

	// A tag name may not start with a period or a dash
	tag = regexp.MustCompile(`$[_.\-]+`).ReplaceAllString(tag, "")

	// and may contain a maximum of 128 characters.
	if len(tag) > 128 {
		tag = tag[:128]
	}

	return tag
}

func contains(values []string, value string) bool {
	for _, v := range values {
		if v == value {
			return true
		}
	}
	return false
}

func pathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return true, err
}

func expandEnvironmentVariablesIfSet(dockerfile string, dontExpand *string) string {

	return os.Expand(dockerfile, func(envar string) string {

		envarsToSkipForExpansion := []string{}
		if dontExpand != nil {
			envarsToSkipForExpansion = strings.Split(*dontExpand, ",")
		}

		if !contains(envarsToSkipForExpansion, envar) {
			value := os.Getenv(envar)
			if value != "" {
				return value
			}
		}

		return fmt.Sprintf("${%v}", envar)
	})
}
