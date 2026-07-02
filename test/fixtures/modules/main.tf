module "vpc_pinned" {
  source = "git::https://github.com/myorg/network-modules.git//vpc?ref=v1.0.0"
}

module "vpc_unpinned" {
  source = "git::https://github.com/myorg/network-modules.git//vpc"
}

module "iam_role" {
  source  = "terraform-aws-modules/iam/aws//modules/iam-assumable-role"
  version = "5.33.0"
}

module "eks" {
  source  = "terraform-aws-modules/eks/aws"
  version = "~> 20.0"
}

module "private_vpc" {
  source  = "app.terraform.io/myorg/vpc/aws"
  version = "2.1.0"
}

module "shared_lib" {
  source = "../shared-modules/logging"
}

module "monitoring" {
  source = "s3::https://s3-eu-west-1.amazonaws.com/consulbucket/monitoring.zip"
}

module "legacy_archive" {
  source = "https://example.com/modules/legacy.zip"
}

# Deliberate collision case: two DIFFERENT git sources sharing the
# literal ref "v1.0.0" — the graph key must NOT collide.
module "collision_a" {
  source = "git::https://github.com/myorg/module-alpha.git?ref=v1.0.0"
}

module "collision_b" {
  source = "git::https://github.com/myorg/module-beta.git?ref=v1.0.0"
}

module "hg_unknown" {
  source = "hg::http://example.com/repo.hg"
}
