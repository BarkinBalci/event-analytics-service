terraform {
  required_version = ">= 1.14.3"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 6.27"
    }
  }

  cloud {
    organization = "barkinbalci"

    workspaces {
      name = "event-analytics-service"
    }
  }
}

provider "aws" {
  region = var.aws_region
}

locals {
  name_prefix = "event-analytics"
  services    = toset(["api", "consumer"])

  common_tags = {
    Project   = "event-analytics-service"
    ManagedBy = "Terraform"
  }

  # Reusable policies and configurations
  ecr_lifecycle_policy = {
    rules = [{
      rulePriority = 1
      description  = "Keep last 10 images"
      selection = {
        tagStatus   = "any"
        countType   = "imageCountMoreThan"
        countNumber = 10
      }
      action = { type = "expire" }
    }]
  }

  ecs_assume_role_policy = {
    Version = "2012-10-17"
    Statement = [{
      Action    = "sts:AssumeRole"
      Effect    = "Allow"
      Principal = { Service = "ecs-tasks.amazonaws.com" }
    }]
  }

  allow_all_egress = {
    description      = "Allow all outbound traffic"
    from_port        = 0
    to_port          = 0
    protocol         = "-1"
    cidr_blocks      = ["0.0.0.0/0"]
    ipv6_cidr_blocks = []
    prefix_list_ids  = []
    security_groups  = []
    self             = false
  }

  clickhouse_secrets = [
    { name = "CLICKHOUSE_HOST", valueFrom = "${aws_secretsmanager_secret.clickhouse.arn}:host::" },
    { name = "CLICKHOUSE_PORT", valueFrom = "${aws_secretsmanager_secret.clickhouse.arn}:port::" },
    { name = "CLICKHOUSE_DATABASE", valueFrom = "${aws_secretsmanager_secret.clickhouse.arn}:database::" },
    { name = "CLICKHOUSE_USER", valueFrom = "${aws_secretsmanager_secret.clickhouse.arn}:user::" },
    { name = "CLICKHOUSE_PASSWORD", valueFrom = "${aws_secretsmanager_secret.clickhouse.arn}:password::" }
  ]

  awslogs_options = {
    "awslogs-region" = var.aws_region
  }

  container_health_check = {
    interval    = 30
    timeout     = 5
    retries     = 3
    startPeriod = 60
  }

  deployment_config = {
    maximum_percent         = 200
    minimum_healthy_percent = 100
  }

  scaling_cooldowns = {
    scale_in_cooldown  = 300
    scale_out_cooldown = 60
  }
}

# VPC
module "vpc" {
  source  = "terraform-aws-modules/vpc/aws"
  version = "~> 6.5"

  name = "${local.name_prefix}-vpc"
  cidr = var.vpc_cidr

  azs             = var.availability_zones
  private_subnets = var.private_subnet_cidrs
  public_subnets  = var.public_subnet_cidrs

  enable_nat_gateway   = true
  single_nat_gateway   = var.single_nat_gateway
  enable_dns_hostnames = true
  enable_dns_support   = true

  tags = local.common_tags
}

# ECR Repositories
resource "aws_ecr_repository" "service" {
  for_each             = local.services
  name                 = "${local.name_prefix}-${each.key}"
  image_tag_mutability = "MUTABLE"

  image_scanning_configuration {
    scan_on_push = true
  }

  tags = local.common_tags
}

resource "aws_ecr_lifecycle_policy" "service" {
  for_each   = local.services
  repository = aws_ecr_repository.service[each.key].name
  policy     = jsonencode(local.ecr_lifecycle_policy)
}

# SQS Queue
resource "aws_sqs_queue" "events" {
  name                       = "${local.name_prefix}-events"
  visibility_timeout_seconds = 60
  message_retention_seconds  = 1209600
  receive_wait_time_seconds  = 20

  redrive_policy = jsonencode({
    deadLetterTargetArn = aws_sqs_queue.events_dlq.arn
    maxReceiveCount     = 3
  })

  tags = local.common_tags
}

resource "aws_sqs_queue" "events_dlq" {
  name                      = "${local.name_prefix}-events-dlq"
  message_retention_seconds = 1209600

  tags = local.common_tags
}

# Secrets Manager
resource "aws_secretsmanager_secret" "clickhouse" {
  name        = "${local.name_prefix}/clickhouse"
  description = "ClickHouse credentials"

  tags = local.common_tags
}

resource "aws_secretsmanager_secret_version" "clickhouse" {
  secret_id = aws_secretsmanager_secret.clickhouse.id
  secret_string = jsonencode({
    host     = var.CLICKHOUSE_HOST
    port     = var.CLICKHOUSE_PORT
    database = var.CLICKHOUSE_DATABASE
    user     = var.CLICKHOUSE_USER
    password = var.CLICKHOUSE_PASSWORD
  })
}

# IAM Roles
resource "aws_iam_role" "ecs_task_execution" {
  name               = "${local.name_prefix}-ecs-task-execution"
  assume_role_policy = jsonencode(local.ecs_assume_role_policy)
  tags               = local.common_tags
}

resource "aws_iam_role_policy_attachment" "ecs_task_execution_policy" {
  role       = aws_iam_role.ecs_task_execution.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AmazonECSTaskExecutionRolePolicy"
}

resource "aws_iam_role_policy" "ecs_secrets_access" {
  name = "${local.name_prefix}-ecs-secrets"
  role = aws_iam_role.ecs_task_execution.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect   = "Allow"
      Action   = ["secretsmanager:GetSecretValue", "kms:Decrypt"]
      Resource = [aws_secretsmanager_secret.clickhouse.arn]
    }]
  })
}

resource "aws_iam_role" "api_task" {
  name               = "${local.name_prefix}-api-task"
  assume_role_policy = jsonencode(local.ecs_assume_role_policy)
  tags               = local.common_tags
}

resource "aws_iam_role_policy" "api_sqs" {
  name = "${local.name_prefix}-api-sqs"
  role = aws_iam_role.api_task.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect   = "Allow"
      Action   = ["sqs:SendMessage", "sqs:GetQueueUrl", "sqs:GetQueueAttributes"]
      Resource = [aws_sqs_queue.events.arn]
    }]
  })
}

resource "aws_iam_role" "consumer_task" {
  name               = "${local.name_prefix}-consumer-task"
  assume_role_policy = jsonencode(local.ecs_assume_role_policy)
  tags               = local.common_tags
}

resource "aws_iam_role_policy" "consumer_sqs" {
  name = "${local.name_prefix}-consumer-sqs"
  role = aws_iam_role.consumer_task.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect = "Allow"
      Action = [
        "sqs:ReceiveMessage",
        "sqs:DeleteMessage",
        "sqs:DeleteMessageBatch",
        "sqs:GetQueueUrl",
        "sqs:GetQueueAttributes",
        "sqs:ChangeMessageVisibility"
      ]
      Resource = [aws_sqs_queue.events.arn]
    }]
  })
}

# ALB
module "alb" {
  source  = "terraform-aws-modules/alb/aws"
  version = "~> 10.4"

  name               = "${local.name_prefix}-alb"
  load_balancer_type = "application"
  vpc_id             = module.vpc.vpc_id
  subnets            = module.vpc.public_subnets
  security_groups    = [aws_security_group.alb.id]

  enable_deletion_protection = false

  listeners = {
    http = {
      port     = 80
      protocol = "HTTP"
      forward  = { target_group_key = "api" }
    }
  }

  target_groups = {
    api = {
      name                 = "${local.name_prefix}-api"
      backend_protocol     = "HTTP"
      backend_port         = 8080
      target_type          = "ip"
      deregistration_delay = 30

      health_check = {
        enabled             = true
        interval            = 30
        path                = "/health"
        port                = "traffic-port"
        protocol            = "HTTP"
        timeout             = 5
        healthy_threshold   = 2
        unhealthy_threshold = 3
        matcher             = "200"
      }

      create_attachment = false
    }
  }

  tags = local.common_tags
}

resource "aws_security_group" "alb" {
  name        = "${local.name_prefix}-alb"
  description = "ALB security group"
  vpc_id      = module.vpc.vpc_id

  ingress {
    from_port   = 80
    to_port     = 80
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  egress = [local.allow_all_egress]
  tags   = local.common_tags
}

# ECS Cluster
module "ecs" {
  source  = "terraform-aws-modules/ecs/aws"
  version = "~> 6.11"

  cluster_name = local.name_prefix

  default_capacity_provider_strategy = {
    FARGATE = {
      weight = 50
    }
    FARGATE_SPOT = {
      weight = 50
    }
  }

  tags = local.common_tags
}

# CloudWatch Log Groups
resource "aws_cloudwatch_log_group" "service" {
  for_each          = local.services
  name              = "/ecs/${local.name_prefix}/${each.key}"
  retention_in_days = var.log_retention_days
  tags              = local.common_tags
}

# Security Groups
resource "aws_security_group" "api" {
  name        = "${local.name_prefix}-api"
  description = "API service"
  vpc_id      = module.vpc.vpc_id

  ingress {
    from_port       = 8080
    to_port         = 8080
    protocol        = "tcp"
    security_groups = [aws_security_group.alb.id]
  }

  egress = [local.allow_all_egress]
  tags   = local.common_tags
}

resource "aws_security_group" "consumer" {
  name        = "${local.name_prefix}-consumer"
  description = "Consumer service"
  vpc_id      = module.vpc.vpc_id
  egress      = [local.allow_all_egress]
  tags        = local.common_tags
}

# ECS Task Definitions
resource "aws_ecs_task_definition" "api" {
  family                   = "${local.name_prefix}-api"
  network_mode             = "awsvpc"
  requires_compatibilities = ["FARGATE"]
  cpu                      = var.api_cpu
  memory                   = var.api_memory
  execution_role_arn       = aws_iam_role.ecs_task_execution.arn
  task_role_arn            = aws_iam_role.api_task.arn

  container_definitions = jsonencode([{
    name      = "api"
    image     = "${aws_ecr_repository.service["api"].repository_url}:${var.image_tag}"
    essential = true

    portMappings = [{ containerPort = 8080, protocol = "tcp" }]

    environment = [
      { name = "SERVICE_ENVIRONMENT", value = "production" },
      { name = "SERVICE_API_PORT", value = "8080" },
      { name = "SERVICE_HOST", value = var.service_host },
      { name = "SQS_QUEUE_URL", value = aws_sqs_queue.events.url },
      { name = "SQS_REGION", value = var.aws_region }
    ]

    secrets = local.clickhouse_secrets

    logConfiguration = {
      logDriver = "awslogs"
      options = merge(local.awslogs_options, {
        "awslogs-group"         = aws_cloudwatch_log_group.service["api"].name
        "awslogs-stream-prefix" = "api"
      })
    }

    healthCheck = merge(local.container_health_check, {
      command = ["CMD-SHELL", "wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1"]
    })
  }])

  tags = local.common_tags
}

resource "aws_ecs_task_definition" "consumer" {
  family                   = "${local.name_prefix}-consumer"
  network_mode             = "awsvpc"
  requires_compatibilities = ["FARGATE"]
  cpu                      = var.consumer_cpu
  memory                   = var.consumer_memory
  execution_role_arn       = aws_iam_role.ecs_task_execution.arn
  task_role_arn            = aws_iam_role.consumer_task.arn

  container_definitions = jsonencode([{
    name      = "consumer"
    image     = "${aws_ecr_repository.service["consumer"].repository_url}:${var.image_tag}"
    essential = true

    environment = [
      { name = "SERVICE_ENVIRONMENT", value = "production" },
      { name = "SQS_QUEUE_URL", value = aws_sqs_queue.events.url },
      { name = "SQS_REGION", value = var.aws_region },
      { name = "CONSUMER_BATCH_SIZE_MIN", value = "100" },
      { name = "CONSUMER_BATCH_SIZE_MAX", value = "2000" },
      { name = "CONSUMER_BATCH_TIMEOUT_SEC", value = "10" },
      { name = "CONSUMER_HEALTH_CHECK_PORT", value = "8081" },
      { name = "CLICKHOUSE_MAX_OPEN_CONNS", value = "10" },
      { name = "CLICKHOUSE_MAX_IDLE_CONNS", value = "5" },
      { name = "CLICKHOUSE_CONN_MAX_LIFETIME_SEC", value = "3600" }
    ]

    secrets = local.clickhouse_secrets

    logConfiguration = {
      logDriver = "awslogs"
      options = merge(local.awslogs_options, {
        "awslogs-group"         = aws_cloudwatch_log_group.service["consumer"].name
        "awslogs-stream-prefix" = "consumer"
      })
    }

    healthCheck = merge(local.container_health_check, {
      command = ["CMD-SHELL", "wget --no-verbose --tries=1 --spider http://localhost:8081/health || exit 1"]
    })
  }])

  tags = local.common_tags
}

# ECS Services
resource "aws_ecs_service" "api" {
  name            = "api"
  cluster         = module.ecs.cluster_id
  task_definition = aws_ecs_task_definition.api.arn
  desired_count   = var.api_desired_count

  capacity_provider_strategy {
    capacity_provider = "FARGATE"
    weight            = 50
    base              = 1
  }

  capacity_provider_strategy {
    capacity_provider = "FARGATE_SPOT"
    weight            = 50
  }

  network_configuration {
    subnets          = module.vpc.private_subnets
    security_groups  = [aws_security_group.api.id]
    assign_public_ip = false
  }

  load_balancer {
    target_group_arn = module.alb.target_groups["api"].arn
    container_name   = "api"
    container_port   = 8080
  }

  deployment_maximum_percent         = local.deployment_config.maximum_percent
  deployment_minimum_healthy_percent = local.deployment_config.minimum_healthy_percent

  tags = local.common_tags

  depends_on = [module.alb]
}

resource "aws_ecs_service" "consumer" {
  name            = "consumer"
  cluster         = module.ecs.cluster_id
  task_definition = aws_ecs_task_definition.consumer.arn
  desired_count   = var.consumer_desired_count

  capacity_provider_strategy {
    capacity_provider = "FARGATE_SPOT"
    weight            = 100
  }

  network_configuration {
    subnets          = module.vpc.private_subnets
    security_groups  = [aws_security_group.consumer.id]
    assign_public_ip = false
  }

  deployment_maximum_percent         = local.deployment_config.maximum_percent
  deployment_minimum_healthy_percent = local.deployment_config.minimum_healthy_percent

  tags = local.common_tags
}

# Auto Scaling
resource "aws_appautoscaling_target" "api" {
  max_capacity       = var.api_max_capacity
  min_capacity       = var.api_min_capacity
  resource_id        = "service/${module.ecs.cluster_name}/${aws_ecs_service.api.name}"
  scalable_dimension = "ecs:service:DesiredCount"
  service_namespace  = "ecs"
}

resource "aws_appautoscaling_policy" "api_cpu" {
  name               = "${local.name_prefix}-api-cpu"
  policy_type        = "TargetTrackingScaling"
  resource_id        = aws_appautoscaling_target.api.resource_id
  scalable_dimension = aws_appautoscaling_target.api.scalable_dimension
  service_namespace  = aws_appautoscaling_target.api.service_namespace

  target_tracking_scaling_policy_configuration {
    target_value = 70.0
    predefined_metric_specification {
      predefined_metric_type = "ECSServiceAverageCPUUtilization"
    }
    scale_in_cooldown  = local.scaling_cooldowns.scale_in_cooldown
    scale_out_cooldown = local.scaling_cooldowns.scale_out_cooldown
  }
}

resource "aws_appautoscaling_target" "consumer" {
  max_capacity       = var.consumer_max_capacity
  min_capacity       = var.consumer_min_capacity
  resource_id        = "service/${module.ecs.cluster_name}/${aws_ecs_service.consumer.name}"
  scalable_dimension = "ecs:service:DesiredCount"
  service_namespace  = "ecs"
}

resource "aws_appautoscaling_policy" "consumer_sqs" {
  name               = "${local.name_prefix}-consumer-sqs"
  policy_type        = "TargetTrackingScaling"
  resource_id        = aws_appautoscaling_target.consumer.resource_id
  scalable_dimension = aws_appautoscaling_target.consumer.scalable_dimension
  service_namespace  = aws_appautoscaling_target.consumer.service_namespace

  target_tracking_scaling_policy_configuration {
    target_value = var.consumer_target_queue_depth

    customized_metric_specification {
      metrics {
        id = "m1"
        metric_stat {
          metric {
            namespace   = "AWS/SQS"
            metric_name = "ApproximateNumberOfMessagesVisible"
            dimensions {
              name  = "QueueName"
              value = aws_sqs_queue.events.name
            }
          }
          stat = "Average"
        }
      }
    }

    scale_in_cooldown  = local.scaling_cooldowns.scale_in_cooldown
    scale_out_cooldown = local.scaling_cooldowns.scale_out_cooldown
  }
}
