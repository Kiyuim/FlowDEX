#!/bin/bash

# Fun DEX Database Import Script
# This script imports all SQL files from docker/fun_dexs into local MySQL

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

# Database configuration
DB_HOST="localhost"
DB_PORT="3306"
DB_USER="root"
DB_PASSWORD="web3ite.fun"
DB_NAME="fun_dexs"
SQL_DIR="docker/fun_dexs"

# MySQL connection command
MYSQL_CMD="mysql -h${DB_HOST} -P${DB_PORT} -u${DB_USER} -p${DB_PASSWORD} ${DB_NAME}"

# Import mode: "drop" to drop existing tables, "skip" to skip existing tables
IMPORT_MODE="drop"

# Function to get table name from SQL file
get_table_name() {
    local file="$1"
    # Extract table name from CREATE TABLE statement
    grep -i "create table" "$file" | head -1 | sed 's/.*create table \([a-zA-Z0-9_]*\).*/\1/I' | tr -d ' '
}

# Function to check if table exists
table_exists() {
    local table_name="$1"
    local result=$(${MYSQL_CMD} -e "SHOW TABLES LIKE '$table_name';" 2>/dev/null | grep -c "$table_name" || echo "0")
    [ "$result" -gt 0 ]
}

# Function to drop table if exists
drop_table() {
    local table_name="$1"
    print_status "Dropping existing table: $table_name"
    ${MYSQL_CMD} -e "DROP TABLE IF EXISTS \`$table_name\`;" 2>/dev/null
}

# Function to execute SQL file
import_sql_file() {
    local file="$1"
    local filename=$(basename "$file")
    
    # Get table name from SQL file
    local table_name=$(get_table_name "$file")
    
    print_status "Processing: $filename (table: $table_name)"
    
    # Check if table exists and handle accordingly
    if table_exists "$table_name"; then
        if [ "$IMPORT_MODE" = "drop" ]; then
            print_warning "Table '$table_name' already exists. Dropping and recreating..."
            drop_table "$table_name"
        elif [ "$IMPORT_MODE" = "skip" ]; then
            print_warning "Table '$table_name' already exists. Skipping..."
            return 0
        fi
    fi
    
    # Execute the SQL file
    if ${MYSQL_CMD} < "$file" 2>/dev/null; then
        print_status "✅ Successfully imported: $filename"
        return 0
    else
        print_error "❌ Failed to import: $filename"
        # Show the actual error
        ${MYSQL_CMD} < "$file" 2>&1 | head -5
        return 1
    fi
}

# Main execution
main() {
    print_header "Fun DEX Database Import"
    
    # Check command line arguments
    if [ "$1" = "--skip-existing" ]; then
        IMPORT_MODE="skip"
        print_status "Mode: Skip existing tables"
    elif [ "$1" = "--drop-existing" ]; then
        IMPORT_MODE="drop"
        print_status "Mode: Drop and recreate existing tables"
    else
        print_status "Mode: Drop and recreate existing tables (default)"
        print_status "Use --skip-existing to skip existing tables"
        print_status "Use --drop-existing to explicitly drop existing tables"
    fi
    
    print_status "Starting database import process..."
    
    # Check if MySQL is running
    if ! systemctl is-active --quiet mysql; then
        print_error "MySQL is not running. Please start MySQL first."
        exit 1
    fi
    
    # Check if SQL directory exists
    if [ ! -d "$SQL_DIR" ]; then
        print_error "SQL directory not found: $SQL_DIR"
        exit 1
    fi
    
    # Test database connection
    print_status "Testing database connection..."
    if ! ${MYSQL_CMD} -e "SELECT 1;" >/dev/null 2>&1; then
        print_error "Cannot connect to MySQL database. Please check your credentials."
        exit 1
    fi
    print_status "✅ Database connection successful"
    
    # Get all SQL files
    sql_files=($(find "$SQL_DIR" -name "*.sql" | sort))
    
    if [ ${#sql_files[@]} -eq 0 ]; then
        print_error "No SQL files found in $SQL_DIR"
        exit 1
    fi
    
    print_status "Found ${#sql_files[@]} SQL files to import"
    
    # Import files in order
    failed_imports=0
    successful_imports=0
    skipped_imports=0
    
    for sql_file in "${sql_files[@]}"; do
        if import_sql_file "$sql_file"; then
            ((successful_imports++))
        else
            ((failed_imports++))
        fi
    done
    
    # Show summary
    print_header "Import Summary:"
    echo -e "  ✅ Successful imports: $successful_imports"
    echo -e "  ❌ Failed imports: $failed_imports"
    echo -e "  📊 Total files processed: $((successful_imports + failed_imports))"
    
    # Show imported tables
    print_header "Imported Tables:"
    ${MYSQL_CMD} -e "SHOW TABLES;" 2>/dev/null | grep -v "Tables_in_" | while read table; do
        if [ -n "$table" ]; then
            echo -e "  📋 $table"
        fi
    done
    
    # Show table counts
    print_header "Table Record Counts:"
    ${MYSQL_CMD} -e "SHOW TABLES;" 2>/dev/null | grep -v "Tables_in_" | while read table; do
        if [ -n "$table" ]; then
            count=$(${MYSQL_CMD} -e "SELECT COUNT(*) FROM \`$table\`;" 2>/dev/null | tail -n 1)
            echo -e "  📊 $table: $count records"
        fi
    done
    
    if [ $failed_imports -eq 0 ]; then
        print_status "🎉 All tables imported successfully!"
    else
        print_warning "Some imports failed. Check the output above for details."
    fi
    
    print_header "Next Steps:"
    echo -e "  1. Update your service configurations to use localhost:3306"
    echo -e "  2. Restart your services"
    echo -e "  3. Test the services with local database"
}

# Check if running from project root
if [ ! -d "$SQL_DIR" ]; then
    print_error "Please run this script from the project root directory"
    print_error "Current directory: $(pwd)"
    print_error "Expected SQL directory: $SQL_DIR"
    exit 1
fi

# Run main function
main "$@" 