# Azure-First Infrastructure Configuration

terraform {
  required_version = ">= 1.0"
  required_providers {
    azurerm = {
      source  = "hashicorp/azurerm"
      version = "~>3.80"
    }
    azuread = {
      source  = "hashicorp/azuread"
      version = "~>2.45"
    }
    random = {
      source  = "hashicorp/random"
      version = "~>3.4"
    }
  }
  
  backend "azurerm" {
    resource_group_name  = "quantumlayer-terraform"
    storage_account_name = "qlpterraformstate"
    container_name      = "tfstate"
    key                 = "infrastructure.terraform.tfstate"
  }
}

provider "azurerm" {
  features {
    key_vault {
      purge_soft_delete_on_destroy    = true
      recover_soft_deleted_key_vaults = true
    }
    resource_group {
      prevent_deletion_if_contains_resources = false
    }
  }
}

provider "azuread" {}

# Variables
variable "environment" {
  description = "Environment name"
  type        = string
  default     = "dev"
}

variable "location" {
  description = "Azure region"
  type        = string
  default     = "UK South"
}

variable "sql_admin_password" {
  description = "SQL Server admin password"
  type        = string
  sensitive   = true
}

variable "openai_api_key" {
  description = "OpenAI API key"
  type        = string
  sensitive   = true
}

variable "mongodb_connection_string" {
  description = "MongoDB Atlas connection string"
  type        = string
  sensitive   = true
}

# Data sources
data "azurerm_client_config" "current" {}

# Random strings for unique naming
resource "random_id" "suffix" {
  byte_length = 4
}

# Resource Groups
resource "azurerm_resource_group" "main" {
  name     = "quantumlayer-${var.environment}"
  location = var.location
  
  tags = {
    Environment   = var.environment
    Project      = "QuantumLayer-UOS"
    CostCenter   = "engineering"
    CreatedBy    = "terraform"
    Owner        = "subrahmanya.gonella"
  }
}

resource "azurerm_resource_group" "networking" {
  name     = "quantumlayer-networking-${var.environment}"
  location = var.location
  
  tags = azurerm_resource_group.main.tags
}

# Virtual Network
resource "azurerm_virtual_network" "main" {
  name                = "qlp-vnet-${var.environment}"
  address_space       = ["10.0.0.0/16"]
  location           = azurerm_resource_group.networking.location
  resource_group_name = azurerm_resource_group.networking.name

  tags = azurerm_resource_group.main.tags
}

# Subnets
resource "azurerm_subnet" "aks" {
  name                 = "aks-subnet"
  resource_group_name  = azurerm_resource_group.networking.name
  virtual_network_name = azurerm_virtual_network.main.name
  address_prefixes     = ["10.0.1.0/24"]
}

resource "azurerm_subnet" "private_endpoints" {
  name                 = "private-endpoints-subnet"
  resource_group_name  = azurerm_resource_group.networking.name
  virtual_network_name = azurerm_virtual_network.main.name
  address_prefixes     = ["10.0.2.0/24"]
}

resource "azurerm_subnet" "application_gateway" {
  name                 = "appgw-subnet"
  resource_group_name  = azurerm_resource_group.networking.name
  virtual_network_name = azurerm_virtual_network.main.name
  address_prefixes     = ["10.0.3.0/24"]
}

# Network Security Groups
resource "azurerm_network_security_group" "aks" {
  name                = "qlp-aks-nsg-${var.environment}"
  location           = azurerm_resource_group.networking.location
  resource_group_name = azurerm_resource_group.networking.name

  security_rule {
    name                       = "AllowHTTPS"
    priority                   = 1001
    direction                  = "Inbound"
    access                     = "Allow"
    protocol                   = "Tcp"
    source_port_range          = "*"
    destination_port_range     = "443"
    source_address_prefix      = "*"
    destination_address_prefix = "*"
  }

  tags = azurerm_resource_group.main.tags
}

# Associate NSG with subnet
resource "azurerm_subnet_network_security_group_association" "aks" {
  subnet_id                 = azurerm_subnet.aks.id
  network_security_group_id = azurerm_network_security_group.aks.id
}

# Log Analytics Workspace
resource "azurerm_log_analytics_workspace" "main" {
  name                = "qlp-logs-${var.environment}-${random_id.suffix.hex}"
  location           = azurerm_resource_group.main.location
  resource_group_name = azurerm_resource_group.main.name
  sku                = "PerGB2018"
  retention_in_days  = 30

  tags = azurerm_resource_group.main.tags
}

# Application Insights
resource "azurerm_application_insights" "main" {
  name                = "qlp-appinsights-${var.environment}"
  location           = azurerm_resource_group.main.location
  resource_group_name = azurerm_resource_group.main.name
  workspace_id       = azurerm_log_analytics_workspace.main.id
  application_type   = "web"

  tags = azurerm_resource_group.main.tags
}

# Key Vault
resource "azurerm_key_vault" "main" {
  name                        = "qlp-kv-${var.environment}-${random_id.suffix.hex}"
  location                   = azurerm_resource_group.main.location
  resource_group_name        = azurerm_resource_group.main.name
  enabled_for_disk_encryption = true
  tenant_id                  = data.azurerm_client_config.current.tenant_id
  soft_delete_retention_days = 7
  purge_protection_enabled   = false
  sku_name                   = "standard"

  access_policy {
    tenant_id = data.azurerm_client_config.current.tenant_id
    object_id = data.azurerm_client_config.current.object_id

    key_permissions = [
      "Get", "List", "Update", "Create", "Import", "Delete", "Recover", "Backup", "Restore"
    ]

    secret_permissions = [
      "Get", "List", "Set", "Delete", "Recover", "Backup", "Restore"
    ]
  }

  tags = azurerm_resource_group.main.tags
}

# Store secrets in Key Vault
resource "azurerm_key_vault_secret" "sql_admin_password" {
  name         = "sql-admin-password"
  value        = var.sql_admin_password
  key_vault_id = azurerm_key_vault.main.id
}

resource "azurerm_key_vault_secret" "openai_api_key" {
  name         = "openai-api-key"
  value        = var.openai_api_key
  key_vault_id = azurerm_key_vault.main.id
}

resource "azurerm_key_vault_secret" "mongodb_connection_string" {
  name         = "mongodb-connection-string"
  value        = var.mongodb_connection_string
  key_vault_id = azurerm_key_vault.main.id
}

# Container Registry
resource "azurerm_container_registry" "main" {
  name                = "qlpacr${var.environment}${random_id.suffix.hex}"
  resource_group_name = azurerm_resource_group.main.name
  location           = azurerm_resource_group.main.location
  sku                = "Standard"
  admin_enabled      = false

  tags = azurerm_resource_group.main.tags
}

# Azure Kubernetes Service
resource "azurerm_kubernetes_cluster" "main" {
  name                = "qlp-aks-${var.environment}"
  location           = azurerm_resource_group.main.location
  resource_group_name = azurerm_resource_group.main.name
  dns_prefix         = "qlp-aks-${var.environment}"

  default_node_pool {
    name                = "default"
    node_count         = 2
    vm_size            = "Standard_D2s_v3"
    vnet_subnet_id     = azurerm_subnet.aks.id
    
    auto_scaling_enabled = true
    min_count           = 1
    max_count           = 5
    
    upgrade_settings {
      max_surge = "10%"
    }
  }

  identity {
    type = "SystemAssigned"
  }

  network_profile {
    network_plugin = "azure"
    service_cidr   = "10.1.0.0/16"
    dns_service_ip = "10.1.0.10"
  }

  oms_agent {
    log_analytics_workspace_id = azurerm_log_analytics_workspace.main.id
  }

  azure_policy_enabled = true

  tags = azurerm_resource_group.main.tags
}

# Grant AKS access to ACR
resource "azurerm_role_assignment" "aks_acr" {
  principal_id                     = azurerm_kubernetes_cluster.main.kubelet_identity[0].object_id
  role_definition_name            = "AcrPull"
  scope                          = azurerm_container_registry.main.id
  skip_service_principal_aad_check = true
}

# SQL Server
resource "azurerm_mssql_server" "main" {
  name                         = "qlp-sql-${var.environment}-${random_id.suffix.hex}"
  resource_group_name          = azurerm_resource_group.main.name
  location                     = azurerm_resource_group.main.location
  version                      = "12.0"
  administrator_login          = "qlpadmin"
  administrator_login_password = var.sql_admin_password

  azuread_administrator {
    login_username = "subrahmanya.gonella"
    object_id      = data.azurerm_client_config.current.object_id
  }

  tags = azurerm_resource_group.main.tags
}

# SQL Database
resource "azurerm_mssql_database" "main" {
  name           = "quantumlayer"
  server_id      = azurerm_mssql_server.main.id
  collation      = "SQL_Latin1_General_CP1_CI_AS"
  sku_name       = "S1"
  zone_redundant = false

  tags = azurerm_resource_group.main.tags
}

# SQL Firewall Rule (allow Azure services)
resource "azurerm_mssql_firewall_rule" "azure_services" {
  name             = "AllowAzureServices"
  server_id        = azurerm_mssql_server.main.id
  start_ip_address = "0.0.0.0"
  end_ip_address   = "0.0.0.0"
}

# Redis Cache
resource "azurerm_redis_cache" "main" {
  name                = "qlp-redis-${var.environment}-${random_id.suffix.hex}"
  location           = azurerm_resource_group.main.location
  resource_group_name = azurerm_resource_group.main.name
  capacity           = 1
  family             = "C"
  sku_name           = "Standard"
  enable_non_ssl_port = false
  minimum_tls_version = "1.2"

  redis_configuration {
    maxmemory_reserved = 50
    maxmemory_delta    = 50
    maxmemory_policy   = "allkeys-lru"
  }

  tags = azurerm_resource_group.main.tags
}

# Storage Account for function apps and general storage
resource "azurerm_storage_account" "main" {
  name                     = "qlpstorage${var.environment}${random_id.suffix.hex}"
  resource_group_name      = azurerm_resource_group.main.name
  location                 = azurerm_resource_group.main.location
  account_tier             = "Standard"
  account_replication_type = "LRS"

  tags = azurerm_resource_group.main.tags
}

# Service Plan for Azure Functions
resource "azurerm_service_plan" "functions" {
  name                = "qlp-functions-plan-${var.environment}"
  resource_group_name = azurerm_resource_group.main.name
  location           = azurerm_resource_group.main.location
  os_type            = "Linux"
  sku_name           = "Y1"  # Consumption plan

  tags = azurerm_resource_group.main.tags
}

# Cognitive Services (for Azure OpenAI)
resource "azurerm_cognitive_account" "openai" {
  name                = "qlp-openai-${var.environment}-${random_id.suffix.hex}"
  location           = "East US"  # Azure OpenAI is not available in UK South yet
  resource_group_name = azurerm_resource_group.main.name
  kind               = "OpenAI"
  sku_name           = "S0"

  tags = azurerm_resource_group.main.tags
}

# API Management
resource "azurerm_api_management" "main" {
  name                = "qlp-apim-${var.environment}-${random_id.suffix.hex}"
  location           = azurerm_resource_group.main.location
  resource_group_name = azurerm_resource_group.main.name
  publisher_name     = "QuantumLayer Platform Ltd"
  publisher_email    = "admin@quantumlayer.dev"
  sku_name           = "Developer_1"

  tags = azurerm_resource_group.main.tags
}

# Outputs
output "resource_group_name" {
  description = "Name of the main resource group"
  value       = azurerm_resource_group.main.name
}

output "aks_cluster_name" {
  description = "Name of the AKS cluster"
  value       = azurerm_kubernetes_cluster.main.name
}

output "aks_cluster_fqdn" {
  description = "FQDN of the AKS cluster"
  value       = azurerm_kubernetes_cluster.main.fqdn
}

output "container_registry_login_server" {
  description = "Login server for the container registry"
  value       = azurerm_container_registry.main.login_server
}

output "sql_server_fqdn" {
  description = "FQDN of the SQL server"
  value       = azurerm_mssql_server.main.fully_qualified_domain_name
}

output "redis_hostname" {
  description = "Hostname of the Redis cache"
  value       = azurerm_redis_cache.main.hostname
}

output "key_vault_uri" {
  description = "URI of the Key Vault"
  value       = azurerm_key_vault.main.vault_uri
}

output "application_insights_instrumentation_key" {
  description = "Instrumentation key for Application Insights"
  value       = azurerm_application_insights.main.instrumentation_key
  sensitive   = true
}

output "openai_endpoint" {
  description = "Endpoint for Azure OpenAI service"
  value       = azurerm_cognitive_account.openai.endpoint
}
