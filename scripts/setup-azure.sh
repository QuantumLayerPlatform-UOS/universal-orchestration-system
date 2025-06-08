#!/bin/bash

# QuantumLayer Platform - Azure Infrastructure Setup
# This script sets up the complete Azure infrastructure for UOS

set -e

echo "🌐 QuantumLayer Platform - Azure Infrastructure Setup"
echo "===================================================="

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Helper functions
log_info() {
    echo -e "${BLUE}ℹ️  $1${NC}"
}

log_success() {
    echo -e "${GREEN}✅ $1${NC}"
}

log_warning() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

log_error() {
    echo -e "${RED}❌ $1${NC}"
}

# Check if command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Check prerequisites
check_prerequisites() {
    log_info "Checking prerequisites..."
    
    local missing_deps=()
    
    if ! command_exists az; then
        missing_deps+=("azure-cli")
    fi
    
    if ! command_exists terraform; then
        missing_deps+=("terraform")
    fi
    
    if ! command_exists kubectl; then
        missing_deps+=("kubectl")
    fi
    
    if ! command_exists helm; then
        missing_deps+=("helm")
    fi
    
    if [ ${#missing_deps[@]} -ne 0 ]; then
        log_error "Missing dependencies:"
        for dep in "${missing_deps[@]}"; do
            echo "  - $dep"
        done
        echo ""
        echo "Please install the missing dependencies and run this script again."
        exit 1
    fi
    
    log_success "All prerequisites are installed"
}

# Azure login and subscription setup
azure_login() {
    log_info "Checking Azure login status..."
    
    if ! az account show >/dev/null 2>&1; then
        log_info "Not logged into Azure. Starting login process..."
        az login
    fi
    
    # List available subscriptions
    log_info "Available Azure subscriptions:"
    az account list --output table
    
    # Prompt for subscription selection if multiple available
    local subscription_count=$(az account list --query "length(@)" --output tsv)
    if [ "$subscription_count" -gt 1 ]; then
        echo ""
        read -p "Enter the subscription ID you want to use: " subscription_id
        az account set --subscription "$subscription_id"
    fi
    
    local current_subscription=$(az account show --query "name" --output tsv)
    log_success "Using Azure subscription: $current_subscription"
}

# Setup Terraform backend
setup_terraform_backend() {
    log_info "Setting up Terraform backend..."
    
    local resource_group="quantumlayer-terraform"
    local storage_account="qlpterraformstate$(date +%s | tail -c 6)"
    local container_name="tfstate"
    local location="UK South"
    
    # Create resource group for Terraform state
    if ! az group show --name "$resource_group" >/dev/null 2>&1; then
        log_info "Creating resource group for Terraform state..."
        az group create --name "$resource_group" --location "$location"
    fi
    
    # Create storage account for Terraform state
    if ! az storage account show --name "$storage_account" --resource-group "$resource_group" >/dev/null 2>&1; then
        log_info "Creating storage account for Terraform state..."
        az storage account create \
            --resource-group "$resource_group" \
            --name "$storage_account" \
            --sku Standard_LRS \
            --encryption-services blob
    fi
    
    # Create container for Terraform state
    local account_key=$(az storage account keys list --resource-group "$resource_group" --account-name "$storage_account" --query '[0].value' -o tsv)
    
    if ! az storage container show --name "$container_name" --account-name "$storage_account" --account-key "$account_key" >/dev/null 2>&1; then
        log_info "Creating container for Terraform state..."
        az storage container create \
            --name "$container_name" \
            --account-name "$storage_account" \
            --account-key "$account_key"
    fi
    
    # Update Terraform configuration with backend details
    cat > infrastructure/terraform/azure/backend.tf << EOF
terraform {
  backend "azurerm" {
    resource_group_name  = "$resource_group"
    storage_account_name = "$storage_account"
    container_name      = "$container_name"
    key                 = "infrastructure.terraform.tfstate"
  }
}
EOF
    
    log_success "Terraform backend configured"
    echo "  Resource Group: $resource_group"
    echo "  Storage Account: $storage_account"
    echo "  Container: $container_name"
}

# Generate secure passwords and API keys
generate_secrets() {
    log_info "Setting up secrets and environment variables..."
    
    # Generate secure SQL password
    local sql_password=$(openssl rand -base64 32 | tr -d "=+/" | cut -c1-25)
    
    # Create terraform.tfvars file
    cat > infrastructure/terraform/azure/terraform.tfvars << EOF
environment = "dev"
location    = "UK South"
sql_admin_password = "$sql_password"
openai_api_key = "\${OPENAI_API_KEY}"
mongodb_connection_string = "\${MONGODB_CONNECTION_STRING}"
EOF
    
    # Create .env file for local development
    cat > .env << EOF
# Azure Configuration
AZURE_SUBSCRIPTION_ID=$(az account show --query "id" --output tsv)
AZURE_TENANT_ID=$(az account show --query "tenantId" --output tsv)

# Database Configuration
SQL_ADMIN_PASSWORD=$sql_password

# AI/ML Configuration (replace with actual keys)
OPENAI_API_KEY=your_openai_api_key_here
ANTHROPIC_API_KEY=your_anthropic_api_key_here

# MongoDB Atlas Configuration (replace with actual connection string)
MONGODB_CONNECTION_STRING=mongodb+srv://username:password@cluster.mongodb.net/

# Local Ollama Configuration
OLLAMA_ENDPOINT=https://your-local-server:11434

# Service Configuration
ORCHESTRATOR_PORT=8001
INTENT_PROCESSOR_PORT=8002
AGENT_MANAGER_PORT=8003
DEPLOYMENT_ENGINE_PORT=8004

# Development
LOG_LEVEL=debug
ENVIRONMENT=development
EOF
    
    log_success "Secrets and environment variables configured"
    log_warning "Please update API keys in .env file before deploying"
}

# Deploy infrastructure with Terraform
deploy_infrastructure() {
    log_info "Deploying Azure infrastructure with Terraform..."
    
    cd infrastructure/terraform/azure
    
    # Initialize Terraform
    log_info "Initializing Terraform..."
    terraform init
    
    # Plan the deployment
    log_info "Planning Terraform deployment..."
    terraform plan -out=tfplan
    
    # Ask for confirmation
    echo ""
    read -p "Do you want to apply this Terraform plan? (y/N): " confirm
    if [[ $confirm != [yY] ]]; then
        log_warning "Deployment cancelled"
        exit 0
    fi
    
    # Apply the deployment
    log_info "Applying Terraform deployment..."
    terraform apply tfplan
    
    cd ../../..
    
    log_success "Infrastructure deployment completed"
}

# Configure kubectl for AKS
configure_kubectl() {
    log_info "Configuring kubectl for AKS..."
    
    local resource_group=$(terraform -chdir=infrastructure/terraform/azure output -raw resource_group_name)
    local cluster_name=$(terraform -chdir=infrastructure/terraform/azure output -raw aks_cluster_name)
    
    # Get AKS credentials
    az aks get-credentials --resource-group "$resource_group" --name "$cluster_name" --overwrite-existing
    
    # Verify connection
    kubectl cluster-info
    
    log_success "kubectl configured for AKS cluster"
}

# Setup monitoring and observability
setup_monitoring() {
    log_info "Setting up monitoring and observability..."
    
    # Add Helm repositories
    helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
    helm repo add grafana https://grafana.github.io/helm-charts
    helm repo add jaegertracing https://jaegertracing.github.io/helm-charts
    helm repo update
    
    # Create monitoring namespace
    kubectl create namespace monitoring --dry-run=client -o yaml | kubectl apply -f -
    
    # Install Prometheus
    helm upgrade --install prometheus prometheus-community/kube-prometheus-stack \
        --namespace monitoring \
        --set grafana.adminPassword=admin123 \
        --wait
    
    # Install Jaeger
    helm upgrade --install jaeger jaegertracing/jaeger \
        --namespace monitoring \
        --wait
    
    log_success "Monitoring stack deployed"
}

# Display deployment information
show_deployment_info() {
    echo ""
    echo "🎉 Azure infrastructure deployment completed!"
    echo "============================================="
    echo ""
    
    local resource_group=$(terraform -chdir=infrastructure/terraform/azure output -raw resource_group_name)
    local aks_fqdn=$(terraform -chdir=infrastructure/terraform/azure output -raw aks_cluster_fqdn)
    local acr_server=$(terraform -chdir=infrastructure/terraform/azure output -raw container_registry_login_server)
    local sql_fqdn=$(terraform -chdir=infrastructure/terraform/azure output -raw sql_server_fqdn)
    local redis_hostname=$(terraform -chdir=infrastructure/terraform/azure output -raw redis_hostname)
    local key_vault_uri=$(terraform -chdir=infrastructure/terraform/azure output -raw key_vault_uri)
    local openai_endpoint=$(terraform -chdir=infrastructure/terraform/azure output -raw openai_endpoint)
    
    echo "🌐 Azure Resources:"
    echo "  • Resource Group:     $resource_group"
    echo "  • AKS Cluster:        $aks_fqdn"
    echo "  • Container Registry: $acr_server"
    echo "  • SQL Server:         $sql_fqdn"
    echo "  • Redis Cache:        $redis_hostname"
    echo "  • Key Vault:          $key_vault_uri"
    echo "  • Azure OpenAI:       $openai_endpoint"
    echo ""
    echo "📊 Monitoring URLs (once ingress is configured):"
    echo "  • Grafana:            https://grafana.your-domain.com"
    echo "  • Prometheus:         https://prometheus.your-domain.com"
    echo "  • Jaeger:             https://jaeger.your-domain.com"
    echo ""
    echo "🔧 Next Steps:"
    echo "  1. Update API keys in .env file"
    echo "  2. Configure your local Ollama server VPN connection"
    echo "  3. Deploy application services: make deploy-dev"
    echo "  4. Configure MongoDB Atlas vector search"
    echo "  5. Set up domain and SSL certificates"
    echo ""
    echo "💰 Cost Monitoring:"
    echo "  • Azure Portal: https://portal.azure.com/#blade/Microsoft_Azure_Billing/ModernBillingMenuBlade/Overview"
    echo "  • Current spend: Check Azure Cost Management"
    echo "  • Budget alerts: Configured for 80% and 95% of £5,000 credit"
    echo ""
    echo "📚 Documentation:"
    echo "  • Azure Architecture: docs/azure-first-architecture.md"
    echo "  • Deployment Guide:   docs/deployment-guide.md"
    echo "  • Monitoring Guide:   docs/monitoring-guide.md"
}

# Main execution
main() {
    check_prerequisites
    azure_login
    setup_terraform_backend
    generate_secrets
    deploy_infrastructure
    configure_kubectl
    setup_monitoring
    show_deployment_info
}

# Parse command line arguments
case "${1:-}" in
    --backend-only)
        log_info "Setting up Terraform backend only"
        check_prerequisites
        azure_login
        setup_terraform_backend
        ;;
    --infrastructure-only)
        log_info "Deploying infrastructure only"
        deploy_infrastructure
        ;;
    --monitoring-only)
        log_info "Setting up monitoring only"
        configure_kubectl
        setup_monitoring
        ;;
    *)
        main
        ;;
esac
