terraform {
  backend "s3" {
    bucket = "myorg-tfstate-dev"
    key    = "eks/terraform.tfstate"
    region = "us-east-1"
  }
}

resource "aws_ecs_cluster" "main" {
  name = "dev-cluster"
}

resource "aws_ecs_service" "api_worker" {
  name            = "api-worker"
  cluster         = aws_ecs_cluster.main.id
  desired_count   = 2
  launch_type     = "FARGATE"
}

resource "aws_ecs_service" "workers" {
  for_each      = { api = { desired_count = 2 }, batch = { desired_count = 1 } }
  name          = each.key
  cluster       = aws_ecs_cluster.main.id
  desired_count = each.value.desired_count
}

resource "aws_iam_role" "task_roles" {
  count = 3
  name  = "task-role-${count.index}"
}

module "vpc" {
  source  = "terraform-aws-modules/vpc/aws"
  version = "~> 5.0"
}
