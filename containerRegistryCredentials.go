package main

// ContainerRegistryCredentials represents the credentials of type container-registry as defined in the server config and passed to this trusted image
type ContainerRegistryCredentials struct {
	Name                 string                                           `json:"name,omitempty"`
	Type                 string                                           `json:"type,omitempty"`
	AdditionalProperties ContainerRegistryCredentialsAdditionalProperties `json:"additionalProperties,omitempty"`
}

// ContainerRegistryCredentialsAdditionalProperties contains the non standard fields for this type of credentials
type ContainerRegistryCredentialsAdditionalProperties struct {
	Repository string `json:"repository,omitempty"`
	Username   string `json:"username,omitempty"`
	Password   string `json:"password,omitempty"`
}
