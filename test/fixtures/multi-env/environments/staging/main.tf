terraform {
  backend "s3" {
    bucket = "myorg-multi-tfstate-staging"
    key    = "app/terraform.tfstate"
    region = "us-east-1"
  }
}

resource "aws_s3_bucket" "data" {
  bucket = "myorg-staging-data"
}
