terraform {
  backend "s3" {
    bucket = "myorg-multi-tfstate-dev"
    key    = "app/terraform.tfstate"
    region = "us-east-1"
  }
}

resource "aws_s3_bucket" "data" {
  bucket = "myorg-dev-data"
}
