#!/bin/bash

# AgentCert Web Frontend Launcher
# This script starts the web frontend with proper configuration

PROJECT_ROOT="/mnt/c/Users/sanjsingh/Downloads/Studies/AgentCert-Framework"
WEB_DIR="$PROJECT_ROOT/chaoscenter/web"
CUSTOM_DIR="/mnt/c/Users/sanjsingh/Downloads/Studies/AgentCert-Framework/local-custom"

# Color codes
BLUE='\033[0;34m'
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m'

echo -e "${BLUE}Starting AgentCert Web Frontend...${NC}\n"

# Check if node_modules exist
if [ ! -d "$WEB_DIR/node_modules" ]; then
    echo -e "${BLUE}Installing dependencies with npm...${NC}"
    cd "$WEB_DIR"
    npm install --legacy-peer-deps --ignore-scripts
    if [ $? -ne 0 ]; then
        echo -e "${RED}Dependency installation failed${NC}"
        exit 1
    fi
fi

# Clean webpack filesystem cache (fixes moved-folder path issues)
if [ -d "$WEB_DIR/node_modules/.cache" ]; then
    echo -e "${BLUE}Clearing webpack cache...${NC}"
    rm -rf "$WEB_DIR/node_modules/.cache"
fi

# Check if certificates exist
if [ ! -d "$WEB_DIR/certificates" ]; then
    echo -e "${BLUE}Generating SSL certificates...${NC}"
    mkdir -p "$WEB_DIR/certificates"
    cd "$WEB_DIR/certificates"
    
    # Generate a simple self-signed certificate for localhost
    openssl req -x509 -newkey rsa:2048 -keyout localhost-key.pem -out localhost.pem -days 365 -nodes \
        -subj "/C=US/ST=State/L=City/O=Org/CN=localhost" 2>/dev/null
    
    if [ $? -ne 0 ]; then
        echo -e "${RED}Certificate generation failed${NC}"
        exit 1
    fi
    echo -e "${GREEN}Certificates generated successfully${NC}"
fi

echo -e "${GREEN}Setup complete!${NC}"
echo -e "${GREEN}To start the dev server on port 2001, run:${NC}"
echo -e "${BLUE}cd $WEB_DIR && npm run dev${NC}\n"
