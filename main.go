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
)

var (
	// flags
	action                     = kingpin.Flag("action", "Any of the following actions: build, push, tag, history.").Envar("ESTAFETTE_EXTENSION_ACTION").String()
	repositories               = kingpin.Flag("repositories", "List of the repositories the image needs to be pushed to or tagged in.").Envar("ESTAFETTE_EXTENSION_REPOSITORIES").String()
	container                  = kingpin.Flag("container", "Name of the container to build, defaults to app label if present.").Envar("ESTAFETTE_EXTENSION_CONTAINER").String()
	tag                        = kingpin.Flag("tag", "Tag for an image to show history for.").Envar("ESTAFETTE_EXTENSION_TAG").String()
	tags                       = kingpin.Flag("tags", "List of tags the image needs to receive.").Envar("ESTAFETTE_EXTENSION_TAGS").String()
	path                       = kingpin.Flag("path", "Directory to build docker container from, defaults to current working directory.").Default(".").Envar("ESTAFETTE_EXTENSION_PATH").String()
	dockerfile                 = kingpin.Flag("dockerfile", "Dockerfile to build, defaults to Dockerfile.").Default("Dockerfile").Envar("ESTAFETTE_EXTENSION_DOCKERFILE").String()
	inlineDockerfile           = kingpin.Flag("inline", "Dockerfile to build inlined.").Envar("ESTAFETTE_EXTENSION_INLINE").String()
	copy                       = kingpin.Flag("copy", "List of files or directories to copy into the build directory.").Envar("ESTAFETTE_EXTENSION_COPY").String()
	args                       = kingpin.Flag("args", "List of build arguments to pass to the build.").Envar("ESTAFETTE_EXTENSION_ARGS").String()
	pushVersionTag             = kingpin.Flag("push-version-tag", "By default the version tag is pushed, so it can be promoted with a release, but if you don't want it you can disable it via this flag.").Default("true").Envar("ESTAFETTE_EXTENSION_PUSH_VERSION_TAG").Bool()
	versionTagPrefix           = kingpin.Flag("version-tag-prefix", "A prefix to add to the version tag so promoting different containers originating from the same pipeline is possible.").Envar("ESTAFETTE_EXTENSION_VERSION_TAG_PREFIX").String()
	versionTagSuffix           = kingpin.Flag("version-tag-suffix", "A suffix to add to the version tag so promoting different containers originating from the same pipeline is possible.").Envar("ESTAFETTE_EXTENSION_VERSION_TAG_SUFFIX").String()
	noCache                    = kingpin.Flag("no-cache", "Indicates cache shouldn't be used when building the image.").Default("false").Envar("ESTAFETTE_EXTENSION_NO_CACHE").Bool()
	noCachePush                = kingpin.Flag("no-cache-push", "Indicates no dlc cache tag should be pushed when building the image.").Default("false").Envar("ESTAFETTE_EXTENSION_NO_CACHE_PUSH").Bool()
	expandEnvironmentVariables = kingpin.Flag("expand-envvars", "By default environment variables get replaced in the Dockerfile, use this flag to disable that behaviour").Default("true").Envar("ESTAFETTE_EXTENSION_EXPAND_VARIABLES").Bool()
	dontExpand                 = kingpin.Flag("dont-expand", "Comma separate list of environment variable names that should not be expanded").Default("PATH").Envar("ESTAFETTE_EXTENSION_DONT_EXPAND").String()

	gitSource = kingpin.Flag("git-source", "Repository source.").Envar("ESTAFETTE_GIT_SOURCE").String()
	gitOwner  = kingpin.Flag("git-owner", "Repository owner.").Envar("ESTAFETTE_GIT_OWNER").String()
	gitName   = kingpin.Flag("git-name", "Repository name, used as application name if not passed explicitly and app label not being set.").Envar("ESTAFETTE_GIT_NAME").String()
	appLabel  = kingpin.Flag("app-name", "App label, used as application name if not passed explicitly.").Envar("ESTAFETTE_LABEL_APP").String()

	minimumSeverityToFail = kingpin.Flag("minimum-severity-to-fail", "Minimum severity of detected vulnerabilities to fail the build on").Default("HIGH").Envar("ESTAFETTE_EXTENSION_SEVERITY").String()

	credentialsPath    = kingpin.Flag("credentials-path", "Path to file with container registry credentials configured at the CI server, passed in to this trusted extension.").Default("/credentials/container_registry.json").String()
	githubAPITokenPath = kingpin.Flag("githubApiToken-path", "Path to file with Github api token credentials configured at the CI server, passed in to this trusted extension.").Default("/credentials/github_api_token.json").String()
)

func main() {

	// parse command line parameters
	kingpin.Parse()

	// init log format from envvar ESTAFETTE_LOG_FORMAT
	foundation.InitLoggingFromEnv(foundation.NewApplicationInfo(appgroup, app, version, branch, revision, buildDate))

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
	// use mounted credential file if present instead of relying on an envvar
	if runtime.GOOS == "windows" {
		*credentialsPath = "C:" + *credentialsPath
	}
	if foundation.FileExists(*credentialsPath) {
		log.Info().Msgf("Reading credentials from file at path %v...", *credentialsPath)
		credentialsFileContent, err := ioutil.ReadFile(*credentialsPath)
		if err != nil {
			log.Fatal().Msgf("Failed reading credential file at path %v.", *credentialsPath)
		}
		err = json.Unmarshal(credentialsFileContent, &credentials)
		if err != nil {
			log.Fatal().Err(err).Msg("Failed unmarshalling injected credentials")
		}
	}

	if runtime.GOOS == "windows" {
		*githubAPITokenPath = "C:" + *githubAPITokenPath
	}
	if foundation.FileExists(*githubAPITokenPath) {
		log.Info().Msgf("Reading credentials from file at path %v...", *githubAPITokenPath)
		credentialsFileContent, err := ioutil.ReadFile(*githubAPITokenPath)
		if err != nil {
			log.Fatal().Msgf("Failed reading credential file at path %v.", *githubAPITokenPath)
		}
		var githubAPIToken []APITokenCredentials
		err = json.Unmarshal(credentialsFileContent, &githubAPIToken)
		if err != nil {
			log.Fatal().Err(err).Msg("Failed unmarshalling injected credentials")
		}
		if len(githubAPIToken) > 0 {
			// set as env, so it gets used by Trivy to avoid github api rate limits when downloading db
			err := os.Setenv("GITHUB_TOKEN", githubAPIToken[0].AdditionalProperties.Token)
			if err != nil {
				log.Fatal().Msgf("Failed reading Github token file at path %v.", *githubAPITokenPath)
			}
		}
	}

	// validate inputs
	validateRepositories(*repositories, *action)

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
			// trim BOM
			sourceDockerfile = strings.TrimPrefix(sourceDockerfile, "\uFEFF")
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

		if len(fromImagePaths) == 0 {

			log.Info().Msgf("%v (as string):", sourceDockerfilePath)
			fmt.Println(targetDockerfile)
			log.Info().Msg("")

			log.Info().Msgf("%v (as bytes):", sourceDockerfilePath)
			data, _ := ioutil.ReadFile(sourceDockerfilePath)
			fmt.Println(data)

			log.Fatal().Msg("Failed detecting image paths in FROM statements, exiting")
		}

		// pull images in advance, so we can log in to different repositories in the same registry (see https://github.com/moby/moby/issues/37569)
		for _, i := range fromImagePaths {
			if i.isOfficialDockerHubImage {
				continue
			}
			loginIfRequired(credentials, false, i.imagePath)
			log.Info().Msgf("Pulling container image %v", i.imagePath)
			pullArgs := []string{
				"pull",
				i.imagePath,
			}
			foundation.RunCommandWithArgs(ctx, "docker", pullArgs)
		}

		// login to registry for destination container image
		containerPath := fmt.Sprintf("%v/%v:%v", repositoriesSlice[0], *container, estafetteBuildVersionAsTag)
		loginIfRequired(credentials, !*noCachePush, containerPath)

		// build docker image
		log.Info().Msgf("Building docker image %v...", containerPath)

		log.Info().Msg("")
		fmt.Println(targetDockerfile)
		log.Info().Msg("")

		// build every layer separately and push it to registry to be used as cache next time
		var dockerLayerCachingPaths []string
		for index, i := range fromImagePaths {
			isFinalLayer := index == len(fromImagePaths)-1
			isCacheable := !*noCache && runtime.GOOS != "windows"
			dockerLayerCachingTag := "dlc"

			if !isFinalLayer {
				if i.stageName == "" || !isCacheable {
					// skip building intermediate layers for caching
					continue
				}
				log.Info().Msgf("Building layer %v...", i.stageName)
				dockerLayerCachingTag = tidyTag(fmt.Sprintf("dlc-%v", i.stageName))
			}

			dockerLayerCachingPath := fmt.Sprintf("%v/%v:%v", repositoriesSlice[0], *container, dockerLayerCachingTag)
			dockerLayerCachingPaths = append(dockerLayerCachingPaths, dockerLayerCachingPath)

			args := []string{
				"build",
			}

			if isCacheable {
				args = append(args, "--build-arg", "BUILDKIT_INLINE_CACHE=1")
				// cache from remote image
				for _, cf := range dockerLayerCachingPaths {
					args = append(args, "--cache-from", cf)
				}
				args = append(args, "--tag", dockerLayerCachingPath)
			} else {
				// disable use of local layer cache
				args = append(args, "--no-cache")
			}

			if isFinalLayer {
				for _, r := range repositoriesSlice {
					args = append(args, "--tag", fmt.Sprintf("%v/%v:%v", r, *container, estafetteBuildVersionAsTag))
					for _, t := range tagsSlice {
						if r == repositoriesSlice[0] && (t == estafetteBuildVersionAsTag || t == dockerLayerCachingTag) {
							continue
						}
						args = append(args, "--tag", fmt.Sprintf("%v/%v:%v", r, *container, t))
					}
				}
			} else {
				args = append(args, "--target", i.stageName)
			}

			// add optional build args
			for _, a := range argsSlice {
				argValue := os.Getenv(a)
				args = append(args, "--build-arg", fmt.Sprintf("%v=%v", a, argValue))
			}

			args = append(args, "--file", targetDockerfilePath)
			args = append(args, *path)
			foundation.RunCommandWithArgs(ctx, "docker", args)

			if isCacheable && !*noCachePush {
				log.Info().Msgf("Pushing cache container image %v", dockerLayerCachingPath)
				pushArgs := []string{
					"push",
					dockerLayerCachingPath,
				}
				foundation.RunCommandWithArgs(ctx, "docker", pushArgs)
			}
		}

		if runtime.GOOS == "windows" {
			return
		}

		// map severity param value to trivy severity
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

		log.Info().Msg("Saving docker image to file for scanning...")
		tmpfile, err := ioutil.TempFile("", "*.tar")
		if err != nil {
			log.Fatal().Err(err).Msg("Failed creating temporary file")
		}

		// Download Trivy db and save it to path /trivy-cache
		bucketName := ""
		for i, _ := range repositoriesSlice {
			if bucketName != credentials[i].AdditionalProperties.TrivyVulnerabilityDBGCSBucket {
				bucketName = credentials[i].AdditionalProperties.TrivyVulnerabilityDBGCSBucket
				foundation.RunCommandWithArgs(ctx, "gsutil", []string{"-m", "cp", "-r", fmt.Sprintf("gs://%v/trivy-cache/*", bucketName), "/trivy-cache"})
			}
		}

		foundation.RunCommandWithArgs(ctx, "docker", []string{"save", containerPath, "-o", tmpfile.Name()})

		// remove .trivyignore file so devs can't game the system
		// if foundation.FileExists(".trivyignore") {
		// 	err = os.Remove(".trivyignore")
		// 	if err != nil {
		// 		log.Fatal().Msg("Could not remove .trivyignore file")
		// 	}
		// }

		log.Info().Msgf("Scanning container image %v for vulnerabilities of severities %v...", containerPath, severityArgument)
		err = foundation.RunCommandWithArgsExtended(ctx, "/trivy", []string{"--cache-dir", "/trivy-cache", "image", "--severity", severityArgument, "--light", "--skip-update", "--no-progress", "--exit-code", "15", "--ignore-unfixed", "--input", tmpfile.Name()})

		if err != nil {
			if strings.EqualFold(err.Error(), "exit status 1") {
				// ignore exit code, until trivy fixes this on their side, see https://github.com/aquasecurity/trivy/issues/8
				// await https://github.com/aquasecurity/trivy/pull/476 to be released
				log.Warn().Msg("Ignoring Unknown OS error")
			} else {
				log.Fatal().Msgf("The container image has vulnerabilities of severity %v! Look at https://estafette.io/usage/fixing-vulnerabilities/ to learn how to fix vulnerabilities in your image.", severityArgument)
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

			// push additional tags
			for _, t := range tagsSlice {

				if r == repositoriesSlice[0] && t == estafetteBuildVersionAsTag {
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

	case "history":

		// minimal using defaults

		// image: extensions/docker:stable
		// action: history
		// repositories:
		// - extensions

		// with defaults:

		// container: ${ESTAFETTE_GIT_NAME}

		// and other paramers:

		// tag: latest

		sourceContainerPath := ""
		if len(repositoriesSlice) > 0 {
			sourceContainerPath += repositoriesSlice[0] + "/"
		}
		sourceContainerPath += *container
		if *tag != "" {
			sourceContainerPath += ":" + *tag
		}

		loginIfRequired(credentials, false, sourceContainerPath)

		log.Info().Msgf("Showing history for container image %v", sourceContainerPath)
		historyArgs := []string{
			"image",
			"history",
			"--human",
			"--no-trunc",
			sourceContainerPath,
		}

		output, err := foundation.GetCommandWithArgsOutput(ctx, "docker", historyArgs)
		if err != nil {
			// pull source container first
			log.Info().Msgf("Pulling container image %v", sourceContainerPath)
			pullArgs := []string{
				"pull",
				sourceContainerPath,
			}
			foundation.RunCommandWithArgs(ctx, "docker", pullArgs)

			foundation.RunCommandWithArgs(ctx, "docker", historyArgs)
		} else {
			log.Info().Msg(output)
		}

	case "dive":

		log.Warn().Msg("Support for 'action: dive' has been removed, please remove your stage")

	case "trivy":

		log.Warn().Msgf("Direct support for 'action: trivy' has been removed, please use 'severity: %v' on the stage with 'action: build' to use a non-default severity", *minimumSeverityToFail)

	default:
		log.Fatal().Msg("Set `action: <action>` on this step to run build, push, tag or history")
	}
}

func validateRepositories(repositories, action string) {
	if repositories == "" && action != "history" {
		log.Fatal().Msg("Set `repositories:` to list at least one `- <repository>` (for example like `- extensions`)")
	}
}

func getCredentialsForContainers(credentials []ContainerRegistryCredentials, containerImages []string) map[string]*ContainerRegistryCredentials {

	filteredCredentialsMap := make(map[string]*ContainerRegistryCredentials)

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

type fromImage struct {
	imagePath                string
	stageName                string
	isOfficialDockerHubImage bool
}

func getFromImagePathsFromDockerfile(dockerfileContent string) ([]fromImage, error) {

	var containerImages []fromImage

	if imagesFromDockerFileRegex == nil {
		imagesFromDockerFileRegex = regexp.MustCompile(`(?mi)^\s*FROM\s+([^\s]+)(\s+AS\s+([^\s]+))?\s*$`)
	}

	matches := imagesFromDockerFileRegex.FindAllStringSubmatch(dockerfileContent, -1)

	log.Debug().Interface("matches", matches).Msg("Showing FROM matches")

	if len(matches) > 0 {
		for _, m := range matches {
			if len(m) > 1 {
				image := m[1]
				stageName := ""
				if len(m) > 3 {
					stageName = m[3]
				}
				containerImages = append(containerImages, fromImage{
					imagePath:                image,
					isOfficialDockerHubImage: strings.Count(image, "/") == 0 || strings.Contains(image, "$"),
					stageName:                stageName,
				})
			}
		}
	}

	log.Info().Msgf("Found %v stages in Dockerfile", len(containerImages))

	return containerImages, nil
}

func loginIfRequired(credentials []ContainerRegistryCredentials, push bool, containerImages ...string) {

	log.Info().Msgf("Filtering credentials for images %v", containerImages)

	// retrieve all credentials
	filteredCredentialsMap := getCredentialsForContainers(credentials, containerImages)

	log.Info().Msgf("Filtered %v container-registry credentials down to %v", len(credentials), len(filteredCredentialsMap))

	if push && len(filteredCredentialsMap) == 0 {
		log.Warn().Msgf("No credentials found for images %v while it's needed for a push. Disable ", containerImages)
	}

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
