package main

// APITokenCredentials represents the credentials of type bitbucket-api-tokene as defined in the server config and passed to this trusted image
type APITokenCredentials struct {
	Name                 string                                  `json:"name,omitempty"`
	Type                 string                                  `json:"type,omitempty"`
	AdditionalProperties APITokenCredentialsAdditionalProperties `json:"additionalProperties,omitempty"`
}

// APITokenCredentialsAdditionalProperties contains the non standard fields for this type of credentials
type APITokenCredentialsAdditionalProperties struct {
	Token string `json:"token,omitempty"`
}
