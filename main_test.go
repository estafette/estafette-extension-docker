package main

import (
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
					Repository: "extensions",
					Username:   "user",
					Password:   "password",
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
					Repository: "extensions",
					Username:   "user",
					Password:   "password",
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
					Repository: "estafette",
					Username:   "user",
					Password:   "password",
				},
			},
			ContainerRegistryCredentials{
				Name: "container-registry-extensions",
				Type: "container-registry",
				AdditionalProperties: ContainerRegistryCredentialsAdditionalProperties{
					Repository: "extensions",
					Username:   "user",
					Password:   "password",
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
					Repository: "estafette",
					Username:   "user",
					Password:   "password",
				},
			},
			ContainerRegistryCredentials{
				Name: "container-registry-extensions",
				Type: "container-registry",
				AdditionalProperties: ContainerRegistryCredentialsAdditionalProperties{
					Repository: "extensions",
					Username:   "user",
					Password:   "password",
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
					Repository: "estafette",
					Username:   "user",
					Password:   "password",
				},
			},
			ContainerRegistryCredentials{
				Name: "container-registry-extensions",
				Type: "container-registry",
				AdditionalProperties: ContainerRegistryCredentialsAdditionalProperties{
					Repository: "extensions",
					Username:   "user",
					Password:   "password",
				},
			},
			ContainerRegistryCredentials{
				Name: "container-registry-gcr-io-estafette",
				Type: "container-registry",
				AdditionalProperties: ContainerRegistryCredentialsAdditionalProperties{
					Repository: "gcr.io/estafette",
					Username:   "user",
					Password:   "password",
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
					Repository: "gcr.io/estafette",
					Username:   "user",
					Password:   "password",
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
					Repository: "eu.gcr.io/estafette",
					Username:   "user",
					Password:   "password",
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
	t.Run("ReturnsNoContainerImagesIfOnlyFromUsesOfficialDockerHubImage", func(t *testing.T) {

		dockerfileContent := "FROM nginx"

		// act
		containerImages, err := getFromImagePathsFromDockerfile([]byte(dockerfileContent))

		assert.Nil(t, err)
		assert.Equal(t, 0, len(containerImages))
	})

	t.Run("ReturnsContainerImageForFromWithoutTag", func(t *testing.T) {

		dockerfileContent := "FROM prom/prometheus\n"

		// act
		containerImages, err := getFromImagePathsFromDockerfile([]byte(dockerfileContent))

		assert.Nil(t, err)
		assert.Equal(t, 1, len(containerImages))
		assert.Equal(t, "prom/prometheus", containerImages[0])
	})

	t.Run("ReturnsContainerImageForFromWithTag", func(t *testing.T) {

		dockerfileContent := "FROM prom/prometheus:latest"

		// act
		containerImages, err := getFromImagePathsFromDockerfile([]byte(dockerfileContent))

		assert.Nil(t, err)
		assert.Equal(t, 1, len(containerImages))
		assert.Equal(t, "prom/prometheus:latest", containerImages[0])
	})

	t.Run("ReturnsContainerImageForFromWithTagAndAsAlias", func(t *testing.T) {

		dockerfileContent := "FROM prom/prometheus:latest AS builder"

		// act
		containerImages, err := getFromImagePathsFromDockerfile([]byte(dockerfileContent))

		assert.Nil(t, err)
		assert.Equal(t, 1, len(containerImages))
		assert.Equal(t, "prom/prometheus:latest", containerImages[0])
	})

	t.Run("ReturnsContainerImageForFromWithTagAndAsAlias", func(t *testing.T) {

		dockerfileContent := "from prom/prometheus:latest as builder"

		// act
		containerImages, err := getFromImagePathsFromDockerfile([]byte(dockerfileContent))

		assert.Nil(t, err)
		assert.Equal(t, 1, len(containerImages))
		assert.Equal(t, "prom/prometheus:latest", containerImages[0])
	})

	t.Run("ReturnsContainerImagesFromMultiStageFile", func(t *testing.T) {

		dockerfileContent := "from prom/prometheus:latest as builder\nRUN somecommand\n\n\nFROM grafana/grafana:6.1.4\n\nCOPY --from=builder /app ."

		// act
		containerImages, err := getFromImagePathsFromDockerfile([]byte(dockerfileContent))

		assert.Nil(t, err)
		assert.Equal(t, 2, len(containerImages))
		assert.Equal(t, "prom/prometheus:latest", containerImages[0])
		assert.Equal(t, "grafana/grafana:6.1.4", containerImages[1])
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
