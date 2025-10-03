#!/bin/bash
set -e # Exit immediately if a command fails

# --- Colors and Styles ---
C_RESET='\033[0m'
C_RED='\033[0;31m'
C_GREEN='\033[0;32m'
C_YELLOW='\033[1;33m'
C_BLUE='\033[0;34m'
C_CYAN='\033[0;36m'
C_BOLD='\033[1m'

# --- Helper Functions ---
print_header() {
    echo -e "${C_CYAN}${C_BOLD}=======================================================${C_RESET}"
    echo -e "${C_CYAN}${C_BOLD} $1${C_RESET}"
    echo -e "${C_CYAN}${C_BOLD}=======================================================${C_RESET}"
}

print_step() {
    echo -e "\n${C_BLUE}>>>${C_RESET} ${C_BOLD}$1${C_RESET}"
}

print_success() {
    echo -e "${C_GREEN}✓ ${1}${C_RESET}"
}

print_warning() {
    echo -e "${C_YELLOW}⚠ ${1}${C_RESET}"
}

print_error() {
    echo -e "${C_RED}✗ ERROR: ${1}${C_RESET}"
}

check_command() {
    command -v "$1" &> /dev/null
}

install_go() {
    print_warning "Go is not installed. Starting Go v${GO_VERSION} installation..."
    $SUDO rm -rf /usr/local/go
    wget "https://golang.org/dl/go${GO_VERSION}.linux-amd64.tar.gz" -O "/tmp/go.tar.gz"
    echo "Unpacking Go..."
    $SUDO tar -C /usr/local -xzf "/tmp/go.tar.gz"
    rm "/tmp/go.tar.gz"

    if ! grep -q "/usr/local/go/bin" "$HOME/.bashrc"; then
        echo -e "\n# Go lang PATH" >> "$HOME/.bashrc"
        echo 'export PATH=$PATH:/usr/local/go/bin' >> "$HOME/.bashrc"
        print_success "Go PATH added to ~/.bashrc."
        
        print_step "Applying changes by sourcing ~/.bashrc..."
        source "$HOME/.bashrc"
    fi

    # Fallback export just in case
    export PATH=$PATH:/usr/local/go/bin
}

# --- Global Variables ---
GO_VERSION="1.21.0" # You can change the Go version here
BACKEND_SESSION_NAME="hoic-backend"
FRONTEND_SESSION_NAME="hoic-frontend"

# --- Script Start ---
clear
echo -e "${C_CYAN}"
cat << "EOF"
  ____                  _      _                  
 / ___|  _   _   _ __   | |    | |    ___     __ _ 
| |     | | | | | '_ \  | |    | |   / _ \   / _` |
| |___  | |_| | | |_) | | |___ | |__| (_) | | (_| |
 \____|  \__,_| | .__/  |_____||_____\___/   \__, |
               |_|                          |___/ 
EOF
echo -e "${C_RESET}"
print_header "Advanced HTTP Request Tool - Installer & Launcher"

# 1. Create backup of main.go on first run
print_step "Performing initial check..."
if [ ! -f "main.go.orig" ]; then
    if [ -f "main.go" ]; then
        print_warning "Creating an original backup of main.go as main.go.orig..."
        cp main.go main.go.orig
        print_success "Backup created."
    else
        print_error "main.go not found! Cannot create a backup. Please ensure the file is present."
        exit 1
    fi
fi

# 2. Warning and Root Check
print_warning "This script will install packages (Go, Screen) and modify system files."
echo "It's recommended to run as root, but a user with sudo privileges will also work."
echo ""
if [ "$(id -u)" -ne 0 ]; then
    print_warning "You are not running as root. Some commands will prompt for a sudo password."
    SUDO="sudo"
else
    SUDO=""
fi
read -p "Do you want to continue? (y/n): " confirm
if [[ "$confirm" != "y" && "$confirm" != "Y" ]]; then
    print_error "Installation cancelled by user."
    exit 1
fi

# 3. System Update
print_step "Performing system update (apt update && apt upgrade)"
$SUDO apt-get update && $SUDO apt-get upgrade -y

# 4. Check & Kill Running Screen Sessions
print_step "Checking for existing Screen sessions..."
if check_command screen && screen -ls | grep -q "$BACKEND_SESSION_NAME"; then
    print_warning "Found running '$BACKEND_SESSION_NAME' session. Terminating..."
    screen -S "$BACKEND_SESSION_NAME" -X quit
fi
if check_command screen && screen -ls | grep -q "$FRONTEND_SESSION_NAME"; then
    print_warning "Found running '$FRONTEND_SESSION_NAME' session. Terminating..."
    screen -S "$FRONTEND_SESSION_NAME" -X quit
fi
print_success "All old sessions have been cleaned up."

# 5. Check & Install Dependencies
print_step "Checking and installing dependencies..."
if ! check_command go; then
    install_go
fi
# Final check for Go
if check_command go; then
    print_success "Go is installed: $(go version)"
else
    print_error "Go installation failed. Please install Go manually and try again."
    exit 1
fi

if ! check_command screen; then
    print_warning "Screen is not installed. Installing..."
    $SUDO apt-get install -y screen
    print_success "Screen installed successfully."
else
    print_success "Screen is already installed."
fi

# 6. Application Configuration
print_step "Application Configuration"
read -p "Enter your Full Name: " FULL_NAME
read -p "Enter a new Username for the panel: " USERNAME
read -s -p "Enter a new Password for the panel: " PASSWORD
echo ""
read -p "Enter the port for the Backend (default: 8080): " PORT_BACKEND
PORT_BACKEND=${PORT_BACKEND:-8080}
read -p "Enter the port for the Frontend (default: 8082): " PORT_FRONTEND
PORT_FRONTEND=${PORT_FRONTEND:-8082}

# 7. Configure Source Files
print_step "Configuring source files (main.go & index.html)..."
# Restore main.go from backup before modifying to ensure sed command works every time
print_warning "Restoring main.go from backup before configuration..."
cp main.go.orig main.go
print_success "main.go restored."

# Modify main.go for user credentials and port. Using '#' as a delimiter to avoid conflicts with passwords.
sed -i "s#\"root\": {Password: \".*\", Name: \".*\", Role: \".*\"}#\"$USERNAME\": {Password: \"$PASSWORD\", Name: \"$FULL_NAME\", Role: \"Administrator\"}#" main.go
sed -i "s/port := \".*\"/port := \"$PORT_BACKEND\"/" main.go
print_success "main.go configured with new user and port."

# Modify index.html for backend port
# First, revert to 8080 to handle multiple runs, then set the new port.
sed -i "s/:[0-9]\{4\}/:8080/g" index.html
sed -i "s/:8080/:$PORT_BACKEND/g" index.html
print_success "index.html configured for backend port."

# 8. Setup Go Project
print_step "Setting up Go modules..."
rm -f go.sum
go mod init go-curl-backend &> /dev/null || true # Ignore error if it already exists
go get github.com/gorilla/websocket

# 9. Run Application
print_step "Launching Backend and Frontend in Screen sessions..."
if ! check_command python3; then
    print_error "python3 is not installed, which is required to serve the frontend."
    print_warning "Please install it with 'sudo apt-get install python3' and run the script again."
    exit 1
fi

screen -dmS "$BACKEND_SESSION_NAME" bash -c 'go run main.go'
screen -dmS "$FRONTEND_SESSION_NAME" bash -c "python3 -m http.server $PORT_FRONTEND"

# 10. Final Summary
sleep 2 # Give a moment for screens to start
if screen -ls | grep -q "$BACKEND_SESSION_NAME" && screen -ls | grep -q "$FRONTEND_SESSION_NAME"; then
    echo ""
    print_header "INSTALLATION & LAUNCH SUCCESSFUL!"
    echo -e "Your application is now running in the background."
    echo ""
    echo -e "   - ${C_YELLOW}Frontend URL:${C_RESET} http://$(curl -s ifconfig.me):$PORT_FRONTEND"
    echo -e "   - ${C_YELLOW}Backend Port:${C_RESET} $PORT_BACKEND"
    echo ""
    print_step "Management & Debug Commands"
    echo -e "   - View Backend logs:    ${C_YELLOW}screen -r $BACKEND_SESSION_NAME${C_RESET}"
    echo -e "   - View Frontend logs:   ${C_YELLOW}screen -r $FRONTEND_SESSION_NAME${C_RESET}"
    echo -e "   - Stop Backend:         ${C_YELLOW}screen -S $BACKEND_SESSION_NAME -X quit${C_RESET}"
    echo -e "   - Stop Frontend:        ${C_YELLOW}screen -S $FRONTEND_SESSION_NAME -X quit${C_RESET}"
    echo -e "   - (To exit a screen session, press ${C_GREEN}CTRL+A${C_RESET} then ${C_GREEN}D${C_RESET})"
    echo ""
else
    print_error "Failed to start one or both services. Please check the logs."
    echo -e "   - To check for backend errors: try running ${C_YELLOW}'go run main.go'${C_RESET} manually."
    echo -e "   - To check for frontend errors: try running ${C_YELLOW}'python3 -m http.server $PORT_FRONTEND'${C_RESET} manually."
fi

