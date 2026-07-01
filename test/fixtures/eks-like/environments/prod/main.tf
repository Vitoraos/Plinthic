terraform {
  backend "s3" {
    bucket = "myorg-tfstate-prod"
    key    = "eks/terraform.tfstate"
    region = "us-east-1"
  }
}

resource "aws_ecs_cluster" "main" {
  name = "prod-cluster"
}

resource "aws_ecs_service" "api_worker" {
  name          = "api-worker"
  cluster       = aws_ecs_cluster.main.id
  desired_count = 5
}

module "vpc" {
  source = "git::https://github.com/myorg/modules.git//vpc?ref=v1.0.0"
}
