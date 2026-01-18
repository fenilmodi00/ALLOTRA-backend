#!/bin/bash

# IPO Backend Deployment Script
# This script handles deployment to staging and production environments

set -e  # Exit on any error

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_NAME="ipo-backend"
DOCKER_COMPOSE_FILE="docker-compose.prod.yml"
ENV_FILE=".env.prod"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Logging functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Help function
show_help() {
    cat << EOF
IPO Backend Deployment Script

Usage: $0 [COMMAND] [OPTIONS]

Commands:
    staging     Deploy to staging environment
    production  Deploy to production environment
    rollback    Rollback to previous version
    status      Check deployment status
    logs        View application logs
    backup      Create database backup
    restore     Restore database from backup
    help        Show this help message

Options:
    --no-backup     Skip database backup (not recommended for production)
    --force         Force deployment without confirmation
    --version TAG   Deploy specific version tag

Examples:
    $0 staging
    $0 production --version v2.0.1
    $0 rollback --force
    $0 backup
    $0 logs --tail 100

EOF
}

# Check prerequisites
check_prerequisites() {
    log_info "Checking prerequisites..."
    
    # Check if Docker is installed and running
    if ! command -v docker &> /dev/null; then
        log_error "Docker is not installed"
        exit 1
    fi
    
    if ! docker info &> /dev/null; then
        log_error "Docker is not running"
        exit 1
    fi
    
    # Check if Docker Compose is installed
    if ! command -v docker-compose &> /dev/null; then
        log_error "Docker Compose is not installed"
        exit 1
    fi
    
    # Check if environment file exists
    if [[ ! -f "$ENV_FILE" ]]; then
        log_warning "Environment file $ENV_FILE not found"
        log_info "Creating template environment file..."
        create_env_template
    fi
    
    log_success "Prerequisites check passed"
}

# Create environment template
create_env_template() {
    cat > "$ENV_FILE" << EOF
# Database Configuration
DB_USER=ipo_user
DB_PASSWORD=your_secure_password_here
DB_NAME=ipo_db
DB_PORT=5432

# Application Configuration
SERVER_PORT=8080
LOG_LEVEL=info

# Redis Configuration (Optional)
REDIS_PORT=6379

# Backup Configuration
BACKUP_DIR=./backups
BACKUP_RETENTION_DAYS=30

# Monitoring Configuration
ENABLE_METRICS=true
METRICS_PORT=9090
EOF
    
    log_warning "Please update $ENV_FILE with your configuration before deploying"
}

# Create database backup
create_backup() {
    local backup_dir="${BACKUP_DIR:-./backups}"
    local timestamp=$(date +"%Y%m%d_%H%M%S")
    local backup_file="$backup_dir/ipo_db_backup_$timestamp.sql"
    
    log_info "Creating database backup..."
    
    # Create backup directory if it doesn't exist
    mkdir -p "$backup_dir"
    
    # Create backup
    docker-compose -f "$DOCKER_COMPOSE_FILE" exec -T db pg_dump -U "${DB_USER:-ipo_user}" "${DB_NAME:-ipo_db}" > "$backup_file"
    
    if [[ $? -eq 0 ]]; then
        log_success "Database backup created: $backup_file"
        
        # Compress backup
        gzip "$backup_file"
        log_success "Backup compressed: $backup_file.gz"
        
        # Clean old backups
        find "$backup_dir" -name "*.sql.gz" -mtime +${BACKUP_RETENTION_DAYS:-30} -delete
        log_info "Old backups cleaned up"
    else
        log_error "Failed to create database backup"
        exit 1
    fi
}

# Deploy to environment
deploy() {
    local environment=$1
    local version=${2:-"latest"}
    local skip_backup=${3:-false}
    local force=${4:-false}
    
    log_info "Starting deployment to $environment environment..."
    
    # Confirmation for production
    if [[ "$environment" == "production" && "$force" != "true" ]]; then
        echo -n "Are you sure you want to deploy to PRODUCTION? (yes/no): "
        read -r confirmation
        if [[ "$confirmation" != "yes" ]]; then
            log_info "Deployment cancelled"
            exit 0
        fi
    fi
    
    # Create backup before deployment (except for staging)
    if [[ "$environment" == "production" && "$skip_backup" != "true" ]]; then
        create_backup
    fi
    
    # Pull latest images
    log_info "Pulling latest Docker images..."
    docker-compose -f "$DOCKER_COMPOSE_FILE" pull
    
    # Build application image
    log_info "Building application image..."
    docker-compose -f "$DOCKER_COMPOSE_FILE" build app
    
    # Stop existing containers
    log_info "Stopping existing containers..."
    docker-compose -f "$DOCKER_COMPOSE_FILE" down
    
    # Start new containers
    log_info "Starting new containers..."
    docker-compose -f "$DOCKER_COMPOSE_FILE" up -d
    
    # Wait for services to be healthy
    log_info "Waiting for services to be healthy..."
    sleep 30
    
    # Health check
    if check_health; then
        log_success "Deployment to $environment completed successfully!"
        
        # Show status
        show_status
    else
        log_error "Deployment failed - services are not healthy"
        log_info "Rolling back..."
        rollback
        exit 1
    fi
}

# Check service health
check_health() {
    local max_attempts=10
    local attempt=1
    
    while [[ $attempt -le $max_attempts ]]; do
        log_info "Health check attempt $attempt/$max_attempts..."
        
        # Check database
        if docker-compose -f "$DOCKER_COMPOSE_FILE" exec -T db pg_isready -U "${DB_USER:-ipo_user}" -d "${DB_NAME:-ipo_db}" &> /dev/null; then
            log_success "Database is healthy"
        else
            log_warning "Database is not ready"
            sleep 10
            ((attempt++))
            continue
        fi
        
        # Check application
        if curl -f http://localhost:${SERVER_PORT:-8080}/health &> /dev/null; then
            log_success "Application is healthy"
            return 0
        else
            log_warning "Application is not ready"
            sleep 10
            ((attempt++))
        fi
    done
    
    log_error "Health check failed after $max_attempts attempts"
    return 1
}

# Show deployment status
show_status() {
    log_info "Deployment Status:"
    echo
    
    # Show running containers
    docker-compose -f "$DOCKER_COMPOSE_FILE" ps
    echo
    
    # Show resource usage
    log_info "Resource Usage:"
    docker stats --no-stream --format "table {{.Container}}\t{{.CPUPerc}}\t{{.MemUsage}}\t{{.NetIO}}\t{{.BlockIO}}"
    echo
    
    # Show recent logs
    log_info "Recent Application Logs:"
    docker-compose -f "$DOCKER_COMPOSE_FILE" logs --tail=10 app
}

# Rollback deployment
rollback() {
    log_warning "Rolling back deployment..."
    
    # Stop current containers
    docker-compose -f "$DOCKER_COMPOSE_FILE" down
    
    # Start with previous image (this is simplified - in real scenario, you'd have versioned images)
    docker-compose -f "$DOCKER_COMPOSE_FILE" up -d
    
    log_success "Rollback completed"
}

# Show logs
show_logs() {
    local service=${1:-"app"}
    local tail=${2:-"100"}
    
    log_info "Showing logs for $service (last $tail lines)..."
    docker-compose -f "$DOCKER_COMPOSE_FILE" logs --tail="$tail" -f "$service"
}

# Restore database from backup
restore_backup() {
    local backup_file=$1
    
    if [[ -z "$backup_file" ]]; then
        log_error "Please specify backup file to restore"
        exit 1
    fi
    
    if [[ ! -f "$backup_file" ]]; then
        log_error "Backup file not found: $backup_file"
        exit 1
    fi
    
    log_warning "This will overwrite the current database!"
    echo -n "Are you sure you want to continue? (yes/no): "
    read -r confirmation
    if [[ "$confirmation" != "yes" ]]; then
        log_info "Restore cancelled"
        exit 0
    fi
    
    log_info "Restoring database from $backup_file..."
    
    # Decompress if needed
    if [[ "$backup_file" == *.gz ]]; then
        gunzip -c "$backup_file" | docker-compose -f "$DOCKER_COMPOSE_FILE" exec -T db psql -U "${DB_USER:-ipo_user}" -d "${DB_NAME:-ipo_db}"
    else
        docker-compose -f "$DOCKER_COMPOSE_FILE" exec -T db psql -U "${DB_USER:-ipo_user}" -d "${DB_NAME:-ipo_db}" < "$backup_file"
    fi
    
    if [[ $? -eq 0 ]]; then
        log_success "Database restored successfully"
    else
        log_error "Failed to restore database"
        exit 1
    fi
}

# Main script logic
main() {
    local command=$1
    shift
    
    # Parse options
    local skip_backup=false
    local force=false
    local version="latest"
    local tail="100"
    
    while [[ $# -gt 0 ]]; do
        case $1 in
            --no-backup)
                skip_backup=true
                shift
                ;;
            --force)
                force=true
                shift
                ;;
            --version)
                version="$2"
                shift 2
                ;;
            --tail)
                tail="$2"
                shift 2
                ;;
            *)
                log_error "Unknown option: $1"
                show_help
                exit 1
                ;;
        esac
    done
    
    # Load environment variables
    if [[ -f "$ENV_FILE" ]]; then
        source "$ENV_FILE"
    fi
    
    case $command in
        staging)
            check_prerequisites
            deploy "staging" "$version" true "$force"
            ;;
        production)
            check_prerequisites
            deploy "production" "$version" "$skip_backup" "$force"
            ;;
        rollback)
            rollback
            ;;
        status)
            show_status
            ;;
        logs)
            show_logs "app" "$tail"
            ;;
        backup)
            create_backup
            ;;
        restore)
            restore_backup "$1"
            ;;
        help|--help|-h)
            show_help
            ;;
        *)
            log_error "Unknown command: $command"
            show_help
            exit 1
            ;;
    esac
}

# Run main function with all arguments
main "$@"