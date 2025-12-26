output "alb_dns_name" {
  description = "ALB DNS name"
  value       = module.alb.dns_name
}

output "ecr_api_repository_url" {
  description = "API ECR repository URL"
  value       = aws_ecr_repository.service["api"].repository_url
}

output "ecr_consumer_repository_url" {
  description = "Consumer ECR repository URL"
  value       = aws_ecr_repository.service["consumer"].repository_url
}

output "sqs_queue_url" {
  description = "SQS queue URL"
  value       = aws_sqs_queue.events.url
}

output "ecs_cluster_name" {
  description = "ECS cluster name"
  value       = module.ecs.cluster_name
}
