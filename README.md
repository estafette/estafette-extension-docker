# estafette-extension-docker
This extension allows you to build, push and tag docker images

* [Usage](#usage)
* [Actions](#actions)
* [Parameters](#parameters)

# Usage

Using this extension we can build, push and tag our container images.

```yaml
bake:
    image: extensions/docker:stable
    action: build
    repositories:
    - eu.gcr.io/travix-com

push-to-docker-registry:
    image: extensions/docker:stable
    action: push
    repositories:
    - eu.gcr.io/travix-com
    when:
    status == 'succeeded' &&
    branch == 'master'
    
tag-container-image:
    image: extensions/docker:stable
    action: tag
    repositories:
        - eu.gcr.io/travix-com
    tags:
        - stable
        - latest    
```

# Actions

The docker extension supports the following actions: `build, push, tag`. For pushing and tagging containers it uses the credentials and trusted images configuration in the Estafette server to get access to Docker registry credentials automatically

## Build

```yaml
bake:
  image: extensions/docker:stable
  action: build
  no-cache: '< bool | false >'
  expand-variables: '< bool | true >'
  container: '< string | ESTAFETTE_GIT_NAME >'
  repositories:
  - estafette
  path: '< string | . >'
  dockerfile: '< string | Dockerfile >'
  copy:
  - < string | copies Dockerfile by default >
  - /etc/ssl/certs/ca-certificates.crt

```
The `no-cache` options ensures that no previous container built for your branch gets used, but it absolutely uses the latest version of the image in your FROM statement.

The `expand-variables` option allows you to turn off variable expansion, in case you use `$PATH` or another frequently set variable in your Dockerfile.  

A minimal version when using all defaults looks like:

```yaml
bake:
  image: extensions/docker:stable
  action: build
  repositories:
  - estafette
```

If you'd like to avoid having a separate Dockerfile you can inline it as well.

```yaml
bake:
    image: extensions/docker:stable
    action: build
    inline: |
      FROM scratch
      COPY ca-certificates.crt /etc/ssl/certs/
      COPY ${ESTAFETTE_GIT_NAME} /
      ENTRYPOINT ["/${ESTAFETTE_GIT_NAME}"]
    repositories:
    - estafette
    path: ./publish
    copy:
    - /etc/ssl/certs/ca-certificates.crt
```

## push

```yaml
push:
  image: extensions/docker:stable
  action: push
  container: '< string | ESTAFETTE_GIT_NAME >'
  repositories:
  - estafette
  tags:
  - < string | tags with ESTAFETTE_BUILD_VERSION by default and is always pushed >
  - dev
```

## tag

To later on tag a specific version with another tag - for example to promote a dev version to stable you can use the docker extension to tag that version with other tags:

```yaml
tag:
  image: extensions/docker:dev
  action: tag
  container: '< string | ESTAFETTE_GIT_NAME >'
  repositories:
  - estafette
  tags:
  - stable
  - latest
```

# Parameters

| Parameter                    | Description                                                                                                                           | Allowed values   | Default value          |
|------------------------------|---------------------------------------------------------------------------------------------------------------------------------------|------------------|------------------------|
| `action`                     | Any of the following actions: build, push, tag, history.                                                                              | build, push, tag |                        |
| `repositories`               | List of the repositories the image needs to be pushed to or tagged in                                                                 |                  |                        |
| `container`                  | Name of the container to build, defaults to app label if present                                                                      |                  | labels:   app: <value> |
| `tag`                        | Tag for an image to show history for                                                                                                  |                  |                        |
| `tags`                       | List of tags the image needs to receive                                                                                               |                  |                        |
| `path`                       | Directory to build docker container from, defaults to current working directory.                                                      |                  | Current Directory      |
| `dockerfile`                 | Dockerfile to build, defaults to Dockerfile                                                                                           |                  | Dockerfile             |
| `inlineDockerfile`           | Dockerfile to build inlined                                                                                                           |                  |                        |
| `copy`                       | List of files or directories to copy into the build directory                                                                         |                  |                        |
| `args`                       | List of build arguments to pass to the build                                                                                          |                  |                        |
| `pushVersionTag`             | By default the version tag is pushed, so it can be promoted with a release, but if you don't want it you can disable it via this flag | true, false      | true                   |
| `versionTagPrefix`           | A prefix to add to the version tag so promoting different containers originating from the same pipeline is possible                   |                  |                        |
| `versionTagSuffix`           | A suffix to add to the version tag so promoting different containers originating from the same pipeline is possible                   |                  |                        |
| `noCache`                    | Indicates cache shouldn't be used when building the image                                                                             | true, false      | false                  |
| `noCachePush`                | Indicates no dlc cache tag should be pushed when building the image                                                                   | true, false      | false                  |
| `expandEnvironmentVariables` | By default environment variables get replaced in the Dockerfile, use this flag to disable that behaviour"                             | true, false      | true                   |
| `dontExpand`                 | Comma separate list of environment variable names that should not be expanded                                                         |                  | PATH                   |
|                              |                                                                                                                                       |                  |                        |