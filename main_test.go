package main

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetCredentialsForContainers(t *testing.T) {
	t.Run("ReturnsEmptyMapIfCredentialsAreEmpty", func(t *testing.T) {

		credentials := []ContainerRegistryCredentials{}
		containerImages := []string{
			"extensions/docker:stable",
		}

		// act
		filteredCredentialsMap := getCredentialsForContainers(credentials, containerImages)

		assert.Equal(t, 0, len(filteredCredentialsMap))
	})

	t.Run("ReturnsEmptyMapIfContainerImagesAreEmpty", func(t *testing.T) {

		credentials := []ContainerRegistryCredentials{
			ContainerRegistryCredentials{
				Name: "container-registry-extensions",
				Type: "container-registry",
				AdditionalProperties: ContainerRegistryCredentialsAdditionalProperties{
					Repository:                    "extensions",
					Username:                      "user",
					Password:                      "password",
					TrivyVulnerabilityDBGCSBucket: "bucket",
					ServiceAccountKeyfile:         "key-file.json",
				},
			},
		}
		containerImages := []string{}

		// act
		filteredCredentialsMap := getCredentialsForContainers(credentials, containerImages)

		assert.Equal(t, 0, len(filteredCredentialsMap))
	})

	t.Run("ReturnsEmptyMapIfNoContainerImagesRepoMatchesCredentialsRepos", func(t *testing.T) {

		credentials := []ContainerRegistryCredentials{
			ContainerRegistryCredentials{
				Name: "container-registry-extensions",
				Type: "container-registry",
				AdditionalProperties: ContainerRegistryCredentialsAdditionalProperties{
					Repository:                    "extensions",
					Username:                      "user",
					Password:                      "password",
					TrivyVulnerabilityDBGCSBucket: "bucket",
					ServiceAccountKeyfile:         "key-file.json",
				},
			},
		}
		containerImages := []string{
			"estafette/estafette-ci-api:1.0.0",
		}

		// act
		filteredCredentialsMap := getCredentialsForContainers(credentials, containerImages)

		assert.Equal(t, 0, len(filteredCredentialsMap))
	})

	t.Run("ReturnsSingleCredentialsIfContainerImagesRepoMatchesCredentialRepos", func(t *testing.T) {

		credentials := []ContainerRegistryCredentials{
			ContainerRegistryCredentials{
				Name: "container-registry-estafette",
				Type: "container-registry",
				AdditionalProperties: ContainerRegistryCredentialsAdditionalProperties{
					Repository:                    "estafette",
					Username:                      "user",
					Password:                      "password",
					TrivyVulnerabilityDBGCSBucket: "bucket",
					ServiceAccountKeyfile:         "key-file.json",
				},
			},
			ContainerRegistryCredentials{
				Name: "container-registry-extensions",
				Type: "container-registry",
				AdditionalProperties: ContainerRegistryCredentialsAdditionalProperties{
					Repository:                    "extensions",
					Username:                      "user",
					Password:                      "password",
					TrivyVulnerabilityDBGCSBucket: "bucket",
					ServiceAccountKeyfile:         "key-file.json",
				},
			},
		}
		containerImages := []string{
			"extensions/docker:stable",
		}

		// act
		filteredCredentialsMap := getCredentialsForContainers(credentials, containerImages)

		assert.Equal(t, 1, len(filteredCredentialsMap))
		assert.Equal(t, "container-registry-extensions", filteredCredentialsMap["extensions"].Name)
	})

	t.Run("ReturnsMultipleCredentialsIfContainerImagesRepoMatchesCredentialRepos", func(t *testing.T) {

		credentials := []ContainerRegistryCredentials{
			ContainerRegistryCredentials{
				Name: "container-registry-estafette",
				Type: "container-registry",
				AdditionalProperties: ContainerRegistryCredentialsAdditionalProperties{
					Repository:                    "estafette",
					Username:                      "user",
					Password:                      "password",
					TrivyVulnerabilityDBGCSBucket: "bucket",
					ServiceAccountKeyfile:         "key-file.json",
				},
			},
			ContainerRegistryCredentials{
				Name: "container-registry-extensions",
				Type: "container-registry",
				AdditionalProperties: ContainerRegistryCredentialsAdditionalProperties{
					Repository:                    "extensions",
					Username:                      "user",
					Password:                      "password",
					TrivyVulnerabilityDBGCSBucket: "bucket",
					ServiceAccountKeyfile:         "key-file.json",
				},
			},
		}
		containerImages := []string{
			"estafette/estafette-ci-api:1.0.0",
			"extensions/docker:stable",
		}

		// act
		filteredCredentialsMap := getCredentialsForContainers(credentials, containerImages)

		assert.Equal(t, 2, len(filteredCredentialsMap))
		assert.Equal(t, "container-registry-extensions", filteredCredentialsMap["extensions"].Name)
		assert.Equal(t, "container-registry-estafette", filteredCredentialsMap["estafette"].Name)
	})

	t.Run("ReturnsMultipleDedupedCredentialsIfContainerImagesRepoMatchesCredentialRepos", func(t *testing.T) {

		credentials := []ContainerRegistryCredentials{
			ContainerRegistryCredentials{
				Name: "container-registry-estafette",
				Type: "container-registry",
				AdditionalProperties: ContainerRegistryCredentialsAdditionalProperties{
					Repository:                    "estafette",
					Username:                      "user",
					Password:                      "password",
					TrivyVulnerabilityDBGCSBucket: "bucket",
					ServiceAccountKeyfile:         "key-file.json",
				},
			},
			ContainerRegistryCredentials{
				Name: "container-registry-extensions",
				Type: "container-registry",
				AdditionalProperties: ContainerRegistryCredentialsAdditionalProperties{
					Repository:                    "extensions",
					Username:                      "user",
					Password:                      "password",
					TrivyVulnerabilityDBGCSBucket: "bucket",
					ServiceAccountKeyfile:         "key-file.json",
				},
			},
			ContainerRegistryCredentials{
				Name: "container-registry-gcr-io-estafette",
				Type: "container-registry",
				AdditionalProperties: ContainerRegistryCredentialsAdditionalProperties{
					Repository:                    "gcr.io/estafette",
					Username:                      "user",
					Password:                      "password",
					TrivyVulnerabilityDBGCSBucket: "bucket",
					ServiceAccountKeyfile:         "key-file.json",
				},
			},
		}
		containerImages := []string{
			"estafette/estafette-ci-web:1.0.0",
			"estafette/estafette-ci-api:1.0.0",
			"extensions/docker:stable",
		}

		// act
		filteredCredentialsMap := getCredentialsForContainers(credentials, containerImages)

		assert.Equal(t, 2, len(filteredCredentialsMap))
		assert.Equal(t, "container-registry-extensions", filteredCredentialsMap["extensions"].Name)
		assert.Equal(t, "container-registry-estafette", filteredCredentialsMap["estafette"].Name)
	})

	t.Run("ReturnsSingleCredentialsIfContainerImagesRepoMatchesCredentialReposForGoogleContainerRegistry", func(t *testing.T) {

		credentials := []ContainerRegistryCredentials{
			ContainerRegistryCredentials{
				Name: "container-registry-gcr-estafette",
				Type: "container-registry",
				AdditionalProperties: ContainerRegistryCredentialsAdditionalProperties{
					Repository:                    "gcr.io/estafette",
					Username:                      "user",
					Password:                      "password",
					TrivyVulnerabilityDBGCSBucket: "bucket",
					ServiceAccountKeyfile:         "key-file.json",
				},
			},
		}
		containerImages := []string{
			"gcr.io/estafette/estafette-ci-web:1.0.0",
		}

		// act
		filteredCredentialsMap := getCredentialsForContainers(credentials, containerImages)

		assert.Equal(t, 1, len(filteredCredentialsMap))
		assert.Equal(t, "container-registry-gcr-estafette", filteredCredentialsMap["gcr.io/estafette"].Name)
	})

	t.Run("ReturnsSingleCredentialsIfContainerImagesRepoMatchesCredentialReposForGoogleContainerRegistry", func(t *testing.T) {

		credentials := []ContainerRegistryCredentials{
			ContainerRegistryCredentials{
				Name: "container-registry-gcr-estafette-eu",
				Type: "container-registry",
				AdditionalProperties: ContainerRegistryCredentialsAdditionalProperties{
					Repository:                    "eu.gcr.io/estafette",
					Username:                      "user",
					Password:                      "password",
					TrivyVulnerabilityDBGCSBucket: "bucket",
					ServiceAccountKeyfile:         "key-file.json",
				},
			},
		}
		containerImages := []string{
			"eu.gcr.io/estafette/estafette-ci-web:1.0.0",
		}

		// act
		filteredCredentialsMap := getCredentialsForContainers(credentials, containerImages)

		assert.Equal(t, 1, len(filteredCredentialsMap))
		assert.Equal(t, "container-registry-gcr-estafette-eu", filteredCredentialsMap["eu.gcr.io/estafette"].Name)
	})
}

func TestGetFromImagePathsFromDockerfile(t *testing.T) {
	t.Run("ReturnContainerImagesIfFromUsesOfficialDockerHubImage", func(t *testing.T) {

		dockerfileContent := `FROM docker:19.03.13

RUN apk update \
    && apk add --no-cache --upgrade \
        git \
    && rm -rf /var/cache/apk/* \
    && git version

LABEL maintainer="estafette.io"

COPY estafette-extension-docker /
COPY ca-certificates.crt /etc/ssl/certs/

ENTRYPOINT ["/estafette-extension-docker"]`

		// act
		containerImages, err := getFromImagePathsFromDockerfile(dockerfileContent)

		assert.Nil(t, err)
		assert.Equal(t, 1, len(containerImages))
		assert.Equal(t, "docker:19.03.13", containerImages[0].imagePath)
		assert.Equal(t, true, containerImages[0].isOfficialDockerHubImage)
	})

	t.Run("ReturnsContainerImageForFromWithoutTag", func(t *testing.T) {

		dockerfileContent := "FROM prom/prometheus\n"

		// act
		containerImages, err := getFromImagePathsFromDockerfile(dockerfileContent)

		assert.Nil(t, err)
		assert.Equal(t, 1, len(containerImages))
		assert.Equal(t, "prom/prometheus", containerImages[0].imagePath)
		assert.Equal(t, false, containerImages[0].isOfficialDockerHubImage)
	})

	t.Run("ReturnsContainerImageForFromWithTag", func(t *testing.T) {

		dockerfileContent := "FROM prom/prometheus:latest"

		// act
		containerImages, err := getFromImagePathsFromDockerfile(dockerfileContent)

		assert.Nil(t, err)
		assert.Equal(t, 1, len(containerImages))
		assert.Equal(t, "prom/prometheus:latest", containerImages[0].imagePath)
		assert.Equal(t, false, containerImages[0].isOfficialDockerHubImage)
	})

	t.Run("ReturnsContainerImageForFromWithTagAndAsAlias", func(t *testing.T) {

		dockerfileContent := "FROM prom/prometheus:latest AS builder"

		// act
		containerImages, err := getFromImagePathsFromDockerfile(dockerfileContent)

		assert.Nil(t, err)
		assert.Equal(t, 1, len(containerImages))
		assert.Equal(t, "prom/prometheus:latest", containerImages[0].imagePath)
		assert.Equal(t, false, containerImages[0].isOfficialDockerHubImage)
	})

	t.Run("ReturnsContainerImageForFromWithTagAndAsAlias", func(t *testing.T) {

		dockerfileContent := "from prom/prometheus:latest as builder"

		// act
		containerImages, err := getFromImagePathsFromDockerfile(dockerfileContent)

		assert.Nil(t, err)
		assert.Equal(t, 1, len(containerImages))
		assert.Equal(t, "prom/prometheus:latest", containerImages[0].imagePath)
		assert.Equal(t, false, containerImages[0].isOfficialDockerHubImage)
	})

	t.Run("ReturnsContainerImagesFromMultiStageFile", func(t *testing.T) {

		dockerfileContent := "from prom/prometheus:latest as builder\nRUN somecommand\n\n\nFROM grafana/grafana:6.1.4\n\nCOPY --from=builder /app ."

		// act
		containerImages, err := getFromImagePathsFromDockerfile(dockerfileContent)

		assert.Nil(t, err)
		assert.Equal(t, 2, len(containerImages))
		assert.Equal(t, "prom/prometheus:latest", containerImages[0].imagePath)
		assert.Equal(t, false, containerImages[0].isOfficialDockerHubImage)
		assert.Equal(t, "builder", containerImages[0].stageName)
		assert.Equal(t, "grafana/grafana:6.1.4", containerImages[1].imagePath)
		assert.Equal(t, false, containerImages[1].isOfficialDockerHubImage)
		assert.Equal(t, "", containerImages[1].stageName)
	})

	t.Run("TrimBomToFindFromPaths", func(t *testing.T) {

		dockerfileBytes := []byte{0xef, 0xbb, 0xbf, 0x46, 0x52, 0x4f, 0x4d, 0x20, 0x6d, 0x63, 0x72, 0x2e, 0x6d, 0x69, 0x63, 0x72, 0x6f, 0x73, 0x6f, 0x66, 0x74, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x64, 0x6f, 0x74, 0x6e, 0x65, 0x74, 0x2f, 0x72, 0x75, 0x6e, 0x74, 0x69, 0x6d, 0x65, 0x2d, 0x64, 0x65, 0x70, 0x73, 0x3a, 0x35, 0x2e, 0x30, 0xa, 0xa, 0x57, 0x4f, 0x52, 0x4b, 0x44, 0x49, 0x52, 0x20, 0x2f, 0x61, 0x70, 0x70, 0xa, 0x43, 0x4f, 0x50, 0x59, 0x20, 0x2e, 0x20, 0x2e, 0x2f, 0xa, 0xa, 0x52, 0x55, 0x4e, 0x20, 0x61, 0x70, 0x74, 0x2d, 0x67, 0x65, 0x74, 0x20, 0x75, 0x70, 0x64, 0x61, 0x74, 0x65, 0x20, 0x5c, 0xa, 0x20, 0x20, 0x20, 0x20, 0x26, 0x26, 0x20, 0x61, 0x70, 0x74, 0x2d, 0x67, 0x65, 0x74, 0x20, 0x69, 0x6e, 0x73, 0x74, 0x61, 0x6c, 0x6c, 0x20, 0x2d, 0x79, 0x20, 0x2d, 0x2d, 0x61, 0x6c, 0x6c, 0x6f, 0x77, 0x2d, 0x75, 0x6e, 0x61, 0x75, 0x74, 0x68, 0x65, 0x6e, 0x74, 0x69, 0x63, 0x61, 0x74, 0x65, 0x64, 0x20, 0x5c, 0xa, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x6c, 0x69, 0x62, 0x63, 0x36, 0x2d, 0x64, 0x65, 0x76, 0x20, 0x5c, 0xa, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x6c, 0x69, 0x62, 0x67, 0x64, 0x69, 0x70, 0x6c, 0x75, 0x73, 0x20, 0x5c, 0xa, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x6c, 0x69, 0x62, 0x78, 0x31, 0x31, 0x2d, 0x64, 0x65, 0x76, 0x20, 0x5c, 0xa, 0x20, 0x20, 0x20, 0x20, 0x20, 0x26, 0x26, 0x20, 0x72, 0x6d, 0x20, 0x2d, 0x72, 0x66, 0x20, 0x2f, 0x76, 0x61, 0x72, 0x2f, 0x6c, 0x69, 0x62, 0x2f, 0x61, 0x70, 0x74, 0x2f, 0x6c, 0x69, 0x73, 0x74, 0x73, 0x2f, 0x2a, 0xa, 0xa, 0x43, 0x4d, 0x44, 0x20, 0x5b, 0x22, 0x64, 0x6f, 0x74, 0x6e, 0x65, 0x74, 0x22, 0x2c, 0x20, 0x22, 0x2e, 0x2f, 0x53, 0x6f, 0x6d, 0x65, 0x2e, 0x64, 0x6c, 0x6c, 0x22, 0x5d, 0xa}
		dockerfileContent := string(dockerfileBytes)

		// trim BOM
		dockerfileContent = strings.TrimPrefix(dockerfileContent, "\uFEFF")

		// act
		containerImages, err := getFromImagePathsFromDockerfile(dockerfileContent)

		assert.Nil(t, err)
		assert.Equal(t, 1, len(containerImages))
		assert.Equal(t, "FROM mcr.microsoft.com/dotnet/runtime-deps:5.0\n\nWORKDIR /app\nCOPY . ./\n\nRUN apt-get update \\\n    && apt-get install -y --allow-unauthenticated \\\n        libc6-dev \\\n        libgdiplus \\\n        libx11-dev \\\n     && rm -rf /var/lib/apt/lists/*\n\nCMD [\"dotnet\", \"./Some.dll\"]\n", dockerfileContent)
		assert.Equal(t, "mcr.microsoft.com/dotnet/runtime-deps:5.0", containerImages[0].imagePath)
		assert.Equal(t, false, containerImages[0].isOfficialDockerHubImage)
	})
}

func TestTidyBuildVersionAsTag(t *testing.T) {
	t.Run("ReturnsBuildVersionIfItContainsOnlyAllowedCharacters", func(t *testing.T) {

		buildVersion := "1.0.23-beta_B"

		// act
		tag := tidyTag(buildVersion)

		assert.Equal(t, "1.0.23-beta_B", tag)
	})

	t.Run("ReturnsSlashReplacedWithDash", func(t *testing.T) {

		buildVersion := "0.0.187-release/release-x"

		// act
		tag := tidyTag(buildVersion)

		assert.Equal(t, "0.0.187-release-release-x", tag)
	})
}
