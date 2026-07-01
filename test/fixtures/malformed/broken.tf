resource "aws_ecs_service" "broken" {
  name = "broken-service"
  cluster = aws_ecs_cluster.main.id
  desired_count = 2
