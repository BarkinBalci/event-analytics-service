variable "aws_region" {
  description = "AWS region"
  type        = string
  default     = "eu-central-1"
}

variable "vpc_cidr" {
  description = "VPC CIDR"
  type        = string
  default     = "10.0.0.0/16"
}

variable "availability_zones" {
  description = "Availability zones"
  type        = list(string)
  default     = ["eu-central-1a", "eu-central-1b"]
}

variable "private_subnet_cidrs" {
  description = "Private subnet CIDRs"
  type        = list(string)
  default     = ["10.0.1.0/24", "10.0.2.0/24"]
}

variable "public_subnet_cidrs" {
  description = "Public subnet CIDRs"
  type        = list(string)
  default     = ["10.0.101.0/24", "10.0.102.0/24"]
}

variable "single_nat_gateway" {
  description = "Use single NAT gateway (cost savings)"
  type        = bool
  default     = true
}

variable "CLICKHOUSE_HOST" {
  description = "ClickHouse host"
  type        = string
}

variable "CLICKHOUSE_PORT" {
  description = "ClickHouse port"
  type        = number
  default     = 9440
}

variable "CLICKHOUSE_DATABASE" {
  description = "ClickHouse database"
  type        = string
  default     = "events"
}

variable "CLICKHOUSE_USER" {
  description = "ClickHouse user"
  type        = string
  default     = "default"
}

variable "CLICKHOUSE_PASSWORD" {
  description = "ClickHouse password"
  type        = string
  sensitive   = true
}

variable "service_host" {
  description = "Service hostname for API"
  type        = string
  default     = "localhost:8080"
}

variable "image_tag" {
  description = "Docker image tag"
  type        = string
  default     = "latest"
}

variable "api_cpu" {
  description = "API task CPU"
  type        = string
  default     = "256"
}

variable "api_memory" {
  description = "API task memory"
  type        = string
  default     = "512"
}

variable "api_desired_count" {
  description = "API desired count"
  type        = number
  default     = 2
}

variable "api_min_capacity" {
  description = "API min capacity"
  type        = number
  default     = 1
}

variable "api_max_capacity" {
  description = "API max capacity"
  type        = number
  default     = 10
}

variable "consumer_cpu" {
  description = "Consumer task CPU"
  type        = string
  default     = "256"
}

variable "consumer_memory" {
  description = "Consumer task memory"
  type        = string
  default     = "512"
}

variable "consumer_desired_count" {
  description = "Consumer desired count"
  type        = number
  default     = 1
}

variable "consumer_min_capacity" {
  description = "Consumer min capacity"
  type        = number
  default     = 1
}

variable "consumer_max_capacity" {
  description = "Consumer max capacity"
  type        = number
  default     = 5
}

variable "consumer_target_queue_depth" {
  description = "Target SQS queue depth per consumer"
  type        = number
  default     = 1000
}

variable "log_retention_days" {
  description = "CloudWatch log retention"
  type        = number
  default     = 30
}
