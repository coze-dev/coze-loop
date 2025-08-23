#!/bin/bash

# Security Setup Script for Coze Loop Project
# This script installs and configures security tools including gitleaks and pre-commit
# Supports macOS, Linux, and Windows (WSL/Git Bash)
# Can be executed from any directory

set -e

echo "🔒 Setting up security tools for Coze Loop project..."

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Function to check if a command exists
command_exists() {
    command -v "$1" &> /dev/null
}

# Function to find Git repository root directory
find_git_root() {
    local current_dir="$PWD"
    local git_root=""

    # Search upwards for .git directory
    while [[ "$current_dir" != "/" ]]; do
        if [[ -d "$current_dir/.git" ]]; then
            git_root="$current_dir"
            break
        fi
        current_dir="$(dirname "$current_dir")"
    done

    if [[ -n "$git_root" ]]; then
        echo "$git_root"
        return 0
    else
        return 1
    fi
}

# Function to find Git repository root directory
find_git_root() {
    local current_dir="$PWD"
    local git_root=""

    # Search upwards for .git directory
    while [[ "$current_dir" != "/" ]]; do
        if [[ -d "$current_dir/.git" ]]; then
            git_root="$current_dir"
            break
        fi
        current_dir="$(dirname "$current_dir")"
    done

    if [[ -n "$git_root" ]]; then
        echo "$git_root"
        return 0
    else
        return 1
    fi
}

# Function to get command version
get_version() {
    local cmd="$1"
    if command_exists "$cmd"; then
        case "$cmd" in
            "gitleaks")
                gitleaks version 2>/dev/null || echo "unknown version"
                ;;
            "pre-commit")
                pre-commit --version 2>/dev/null | head -n1 || echo "unknown version"
                ;;
            *)
                echo "unknown version"
                ;;
        esac
    else
        echo "not installed"
    fi
}

# Function to install gitleaks
install_gitleaks() {
    local platform="$1"

    if command_exists gitleaks; then
        local version=$(get_version gitleaks)
        print_success "Gitleaks already installed: $version"
        return 0
    fi

    print_status "Installing gitleaks..."

    case "$platform" in
        "macos")
            if command_exists brew; then
                brew install gitleaks
            else
                print_error "Homebrew is not installed. Please install it first:"
                echo "  /bin/bash -c \"\$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)\""
                return 1
            fi
            ;;
        "linux")
            if command_exists apt-get; then
                sudo apt-get update
                sudo apt-get install -y gitleaks
            elif command_exists yum; then
                sudo yum install -y gitleaks
            elif command_exists dnf; then
                sudo dnf install -y gitleaks
            elif command_exists snap; then
                sudo snap install gitleaks
            else
                print_error "Unsupported package manager. Please install gitleaks manually."
                return 1
            fi
            ;;
        "windows")
            if command_exists winget; then
                winget install gitleaks.gitleaks
            elif command_exists chocolatey; then
                choco install gitleaks
            elif command_exists scoop; then
                scoop install gitleaks
            else
                print_error "No supported package manager found on Windows."
                print_status "Please install one of: winget, chocolatey, or scoop"
                print_status "Or download from: https://github.com/gitleaks/gitleaks/releases"
                return 1
            fi
            ;;
    esac

    # Verify installation
    if command_exists gitleaks; then
        local version=$(get_version gitleaks)
        print_success "Gitleaks installed successfully: $version"
        return 0
    else
        print_error "Gitleaks installation failed"
        return 1
    fi
}

# Function to install pre-commit
install_pre_commit() {
    local platform="$1"

    if command_exists pre-commit; then
        local version=$(get_version pre-commit)
        print_success "Pre-commit already installed: $version"
        return 0
    fi

    print_status "Installing pre-commit..."

    case "$platform" in
        "macos")
            if command_exists brew; then
                brew install pre-commit
            else
                print_error "Homebrew is not installed. Please install it first:"
                echo "  /bin/bash -c \"\$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)\""
                return 1
            fi
            ;;
        "linux")
            if command_exists apt-get; then
                sudo apt-get update
                sudo apt-get install -y pre-commit
            elif command_exists yum; then
                sudo yum install -y pre-commit
            elif command_exists dnf; then
                sudo dnf install -y pre-commit
            elif command_exists snap; then
                sudo snap install pre-commit
            else
                # Try pip as fallback
                if command_exists pip3; then
                    pip3 install pre-commit
                elif command_exists pip; then
                    pip install pre-commit
                else
                    print_error "No supported package manager or pip found. Please install pre-commit manually."
                    return 1
                fi
            fi
            ;;
        "windows")
            if command_exists winget; then
                winget install pre-commit.pre-commit
            elif command_exists chocolatey; then
                choco install pre-commit
            elif command_exists scoop; then
                scoop install pre-commit
            else
                # Try pip as fallback
                if command_exists pip3; then
                    pip3 install pre-commit
                elif command_exists pip; then
                    pip install pre-commit
                else
                    print_error "No supported package manager or pip found. Please install pre-commit manually."
                    return 1
                fi
            fi
            ;;
    esac

    # Verify installation
    if command_exists pre-commit; then
        local version=$(get_version pre-commit)
        print_success "Pre-commit installed successfully: $version"
        return 0
    else
        print_error "Pre-commit installation failed"
        return 1
    fi
}

# Detect operating system and platform
detect_platform() {
    if [[ "$OSTYPE" == "darwin"* ]]; then
        echo "macos"
    elif [[ "$OSTYPE" == "linux-gnu"* ]] || [[ "$OSTYPE" == "linux-musl"* ]]; then
        echo "linux"
    elif [[ "$OSTYPE" == "msys" ]] || [[ "$OSTYPE" == "cygwin" ]] || [[ "$OSTYPE" == "win32" ]]; then
        echo "windows"
    else
        # Additional Windows detection
        if [[ "$OS" == "Windows_NT" ]] || [[ "$(uname -s)" == "MINGW"* ]] || [[ "$(uname -s)" == "MSYS"* ]]; then
            echo "windows"
        else
            echo "unknown"
        fi
    fi
}

# Main installation logic
main() {
    local platform=$(detect_platform)
    local script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
    local current_dir="$PWD"
    local git_root=""

    # Find Git repository root
    print_status "Looking for Git repository..."
    if git_root=$(find_git_root); then
        print_success "Found Git repository at: $git_root"

        # Check if we need to change to git root directory
        if [[ "$current_dir" != "$git_root" ]]; then
            print_status "Changing to Git repository root directory..."
            cd "$git_root"
            print_success "Now working in: $(pwd)"
        fi
    else
        print_warning "No Git repository found in current directory or parent directories"
        print_status "Continuing with tool installation only..."
    fi

    case "$platform" in
        "macos")
            print_status "Detected macOS, using Homebrew for installation..."
            ;;
        "linux")
            print_status "Detected Linux, using package manager for installation..."
            ;;
        "windows")
            print_status "Detected Windows, using available package managers..."
            ;;
        "unknown")
            print_error "Unsupported operating system: $OSTYPE"
            print_status "Please install gitleaks and pre-commit manually:"
            echo "  - Gitleaks: https://github.com/gitleaks/gitleaks#installation"
            echo "  - Pre-commit: https://pre-commit.com/#installation"
            exit 1
            ;;
    esac

    # Install tools
    install_gitleaks "$platform"
    install_pre_commit "$platform"

    # Install pre-commit hooks only if we're in a Git repository
    if [[ -n "$git_root" ]]; then
        print_status "Installing pre-commit hooks..."
        if command_exists pre-commit; then
            # Check if .pre-commit-config.yaml exists
            if [[ -f ".pre-commit-config.yaml" ]]; then
                pre-commit install --install-hooks
                print_success "Pre-commit hooks installed"
            else
                print_warning ".pre-commit-config.yaml not found, skipping hook installation"
                print_status "Please ensure you're in the correct repository or create the config file"
            fi
        else
            print_error "Pre-commit not available for hook installation"
            return 1
        fi
    else
        print_warning "Skipping pre-commit hook installation (not in Git repository)"
    fi

    # Test gitleaks configuration
    print_status "Testing gitleaks configuration..."
    if command_exists gitleaks; then
        if [[ -f ".gitleaks.toml" ]]; then
            if gitleaks detect --source . --config .gitleaks.toml --verbose --no-git 2>/dev/null; then
                print_success "Gitleaks configuration test passed"
            else
                print_warning "Gitleaks configuration test had issues (this might be normal for existing repos)"
            fi
        else
            print_warning ".gitleaks.toml not found, skipping configuration test"
            print_status "You may need to create this file or run the script from the repository root"
        fi
    fi

    # Create .gitleaksignore if it doesn't exist and we're in a Git repository
    if [[ -n "$git_root" ]] && [[ ! -f ".gitleaksignore" ]]; then
        print_status "Creating .gitleaksignore file..."
        cat > .gitleaksignore << 'EOF'
# Gitleaks ignore file
# Add patterns here to ignore false positives

# Example patterns:
# ^.*\.md$                    # Ignore all markdown files
# ^.*\.txt$                   # Ignore all text files
# ^.*/test/.*$               # Ignore test directories
# ^.*/examples/.*$           # Ignore example directories

# Add your project-specific ignore patterns below:
EOF
        print_success "Created .gitleaksignore file"
    fi

    # Test pre-commit hooks only if we're in a Git repository
    if [[ -n "$git_root" ]] && command_exists pre-commit; then
        print_status "Testing pre-commit hooks..."
        if pre-commit run --all-files 2>/dev/null; then
            print_success "Pre-commit hooks test passed"
        else
            print_warning "Pre-commit hooks test had issues (check the output above)"
        fi
    fi

    # Return to original directory if we changed it
    if [[ -n "$git_root" ]] && [[ "$current_dir" != "$git_root" ]]; then
        cd "$current_dir"
        print_status "Returned to original directory: $(pwd)"
    fi

    print_success "Security tools setup completed!"
    echo ""
    echo "📋 Next steps:"
    if [[ -n "$git_root" ]]; then
        echo "1. Review the .gitleaks.toml configuration file"
        echo "2. Customize .gitleaksignore if needed"
        echo "3. Commit your changes to enable the pre-commit hooks"
        echo "4. Run 'gitleaks detect --source .' to scan your entire repository"
    else
        echo "1. Navigate to your Git repository root directory"
        echo "2. Run this script again to install pre-commit hooks"
        echo "3. Create .gitleaks.toml configuration file if needed"
    fi
    echo ""
    echo "🔗 Useful commands:"
    echo "  - gitleaks detect --source .                    # Scan entire repo"
    echo "  - gitleaks detect --source . --verbose          # Verbose scan"
    if [[ -n "$git_root" ]]; then
        echo "  - pre-commit run --all-files                   # Run all hooks"
        echo "  - pre-commit run gitleaks                      # Run only gitleaks hook"
    fi
    echo ""
    echo "📚 Documentation:"
    echo "  - Gitleaks: https://github.com/gitleaks/gitleaks"
    echo "  - Pre-commit: https://pre-commit.com/"
}

# Run main function
main "$@"
