#!/bin/bash

set -e

ENV_FILES=(
    "./.env"
    "./scripts/.env"
    "../.env"
    "$HOME/.env"
)

TOKEN=""

for env_file in "${ENV_FILES[@]}"; do
    if [ -f "$env_file" ]; then
        echo "Checking $env_file for GitHub token..."
        while IFS='=' read -r key value; do
            key=$(echo "$key" | xargs)
            value=$(echo "$value" | xargs | sed "s/^['\"]//;s/['\"]$//")
            
            if [[ "$key" =~ ^(GITHUB_TOKEN|GH_TOKEN|GIT_TOKEN)$ ]] && [ -n "$value" ]; then
                TOKEN="$value"
                echo "Found token in $env_file: $key"
                break
            fi
        done < <(grep -v '^#' "$env_file" | grep -v '^$' | grep -iE "^(GITHUB_TOKEN|GH_TOKEN|GIT_TOKEN)=")
        
        if [ -n "$TOKEN" ]; then
            break
        fi
    fi
done

if [ -z "$TOKEN" ] && [ -n "$GITHUB_TOKEN" ]; then
    TOKEN="$GITHUB_TOKEN"
    echo "Using GITHUB_TOKEN from environment"
fi

if [ -z "$TOKEN" ] && [ -n "$GH_TOKEN" ]; then
    TOKEN="$GH_TOKEN"
    echo "Using GH_TOKEN from environment"
fi

if [ -z "$TOKEN" ]; then
    echo "Error: GitHub token not found"
    echo "Please set GITHUB_TOKEN, GH_TOKEN, or GIT_TOKEN environment variable"
    echo "Or ensure one of these files contains the token:"
    printf '  - %s\n' "${ENV_FILES[@]}"
    exit 1
fi

echo "Configuring Git to use token for GitHub access..."
git config --global url."https://${TOKEN}@github.com/".insteadOf "https://github.com/"

echo "Setting GOPRIVATE and GONOSUMDB..."
export GOPRIVATE="github.com/mocachain/*"
export GONOSUMDB="github.com/mocachain/*"

export GITHUB_TOKEN="$TOKEN"

echo ""
echo "Configuration complete!"
echo "GOPRIVATE=$GOPRIVATE"
echo "GONOSUMDB=$GONOSUMDB"
echo ""
echo "You can now run: go mod verify"
