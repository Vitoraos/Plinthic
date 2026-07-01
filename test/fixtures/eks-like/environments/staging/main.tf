terraform {
  backend "s3" {
    bucket = "myorg-tfstate-staging"
    key    = "eks/terraform.tfstate"
    region = "us-east-1"
  }
}

resource "aws_ecs_cluster" "main" {
  name = "staging-cluster"
}

resource "aws_ecs_service" "api_worker" {
  name          = "api-worker"
  cluster       = "staging-cluster"
  desired_count = 2
}

module "vpc" {
  source = "git::https://github.com/myorg/modules.git//vpc?ref=v2.3.1"
}
