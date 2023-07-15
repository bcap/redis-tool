#!/bin/bash

set -e -u -o pipefail

cd $(dirname $0)

if git status -s | grep -q .; then
    echo "dirty repo, run git status"
    exit 2
fi

CURRENT="$(cat VERSION)"

echo "Current version:   $CURRENT"

if [[ "$#" > 0 ]]; then
    VERSION="$1"
else
    read -p "Enter new version: " VERSION
fi

if [ "$VERSION" == "$CURRENT" ]; then
    echo "input version is the same as current"
    exit 2
fi

VERSION_REGEX='^v[0-9][0-9]*\.[0-9][0-9]*\.[0-9][0-9]*(-[a-zA-Z0-9]*)?$'

if ! echo $VERSION | grep -q -E "$VERSION_REGEX"; then 
    echo "invalid version string $VERSION"
    exit 2
fi

echo "Changing version from $CURRENT to $VERSION"
echo "The next steps will build the project and push data to both docker and github"
read -p "Continue? [y/n]: " CONTINUE
if [ "$CONTINUE" != "y" ]; then
    echo "user aborted"
    exit 2
fi

echo -n $VERSION > VERSION

docker buildx build --push --platform linux/arm64,linux/amd64 --tag bcap/redis-tools:$VERSION .

git add VERSION
git commit -m "Release $VERSION"
git tag $VERSION
git push --tags
