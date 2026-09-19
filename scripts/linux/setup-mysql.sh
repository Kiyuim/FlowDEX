#!/bin/bash

# Fun DEX MySQL Setup Script
# This script sets up MySQL with the required credentials and database

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

print_header() {
    echo -e "${BLUE}[HEADER]${NC} $1"
}

# Database configuration (matching your current setup)
DB_PASSWORD="web3ite.fun"
DB_NAME="fun_dexs"

# Main execution
main() {
    print_header "Fun DEX MySQL Local Setup"
    print_status "Setting up MySQL with your existing credentials..."
    
    # Check if MySQL is running
    if ! systemctl is-active --quiet mysql; then
        print_error "MySQL is not running. Starting MySQL..."
        sudo systemctl start mysql
        sleep 2
    fi
    
    print_status "MySQL is running"
    
    # Set root password and create database
    print_status "Configuring MySQL root password and creating database..."
    
    # Create a temporary SQL script
    cat > /tmp/mysql_setup.sql << EOF
-- Set root password
ALTER USER 'root'@'localhost' IDENTIFIED WITH mysql_native_password BY '${DB_PASSWORD}';

-- Create database
CREATE DATABASE IF NOT EXISTS ${DB_NAME} CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

-- Show databases to confirm
SHOW DATABASES;

-- Grant all privileges to root (if needed)
GRANT ALL PRIVILEGES ON *.* TO 'root'@'localhost' WITH GRANT OPTION;
FLUSH PRIVILEGES;
EOF
    
    # Execute the SQL script
    if sudo mysql < /tmp/mysql_setup.sql; then
        print_status "MySQL configuration completed successfully!"
    else
        print_error "Failed to configure MySQL"
        rm -f /tmp/mysql_setup.sql
        exit 1
    fi
    
    # Clean up temporary file
    rm -f /tmp/mysql_setup.sql
    
    # Test the connection
    print_status "Testing MySQL connection..."
    if mysql -u root -p"${DB_PASSWORD}" -e "USE ${DB_NAME}; SELECT 'Connection successful!' as status;" 2>/dev/null; then
        print_status "✅ Connection test successful!"
    else
        print_warning "Connection test failed, but configuration should be correct"
    fi
    
    print_header "MySQL Setup Summary:"
    echo -e "  📊 Database: ${DB_NAME}"
    echo -e "  👤 Username: root"
    echo -e "  🔑 Password: ${DB_PASSWORD}"
    echo -e "  🌐 Host: localhost"
    echo -e "  🔌 Port: 3306"
    echo -e ""
    print_status "You can now update your service configurations to use localhost"
    
    print_header "Next Steps:"
    echo -e "  1. Update configuration files to use localhost:3306"
    echo -e "  2. Import database schema (if needed)"
    echo -e "  3. Restart your services"
}

# Check if running as root or with sudo access
if [ "$EUID" -eq 0 ]; then
    print_error "Please don't run this script as root. Run as regular user with sudo access."
    exit 1
fi

# Check if MySQL is installed
if ! command -v mysql &> /dev/null; then
    print_error "MySQL is not installed. Please install it first with:"
    print_error "sudo apt install mysql-server"
    exit 1
fi

# Run main function
main 