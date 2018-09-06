package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTidyBuildVersionAsTag(t *testing.T) {
	t.Run("ReturnsBuildVersionIfItContainsOnlyAllowedCharacters", func(t *testing.T) {

		buildVersion := "1.0.23-beta_B"

		// act
		tag := tidyBuildVersionAsTag(buildVersion)

		assert.Equal(t, "1.0.23-beta_B", tag)
	})

	t.Run("ReturnsSlashReplacedWithDash", func(t *testing.T) {

		buildVersion := "0.0.187-release/release-x"

		// act
		tag := tidyBuildVersionAsTag(buildVersion)

		assert.Equal(t, "0.0.187-release-release-x", tag)
	})

}
