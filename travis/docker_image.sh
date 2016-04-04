#!/bin/bash

DOCKER_TAG=$(([ "${TRAVIS_BRANCH}" = "master" ] && echo latest) || ([ "${TRAVIS_BRANCH}" = "v1" ] && echo v1))

if [ -n "${DOCKER_TAG}" ] && [ "${TRAVIS_GO_VERSION}" = "1.6" ]; then
  cat > ~/.dockercfg <<EOF
{
  "https://index.docker.io/v1/": {
    "auth": "${HUB_AUTH}",
    "email": "${HUB_EMAIL}"
  }
}
EOF
  docker build -t tsuru/archive-server:${DOCKER_TAG} .
  docker push tsuru/archive-server:${DOCKER_TAG}
else
  echo "No image to build"
fi
