output "vpc_id" {
  description = "The ID of the VPC."
  value       = aws_vpc.this.id
}

output "availability_zones" {
  description = "The Availability Zones of the subnets, in the order given."
  value       = var.availability_zones
}

output "public_subnet_ids" {
  description = "The IDs of the public subnets, one for each zone, in the order of the zones: for the load balancer of the web front end."
  value       = [for zone in var.availability_zones : aws_subnet.public[zone].id]
}

output "private_subnet_ids" {
  description = "The IDs of the private subnets, one for each zone, in the order of the zones: for the tasks of the contexts and of the web front end."
  value       = [for zone in var.availability_zones : aws_subnet.private[zone].id]
}

output "database_subnet_ids" {
  description = "The IDs of the database subnets, one for each zone, in the order of the zones: for the Aurora cluster."
  value       = [for zone in var.availability_zones : aws_subnet.database[zone].id]
}

output "endpoints_security_group_id" {
  description = "The ID of the security group of the interface endpoints. A task reaches the AWS services when its security group allows HTTPS to this group."
  value       = aws_security_group.endpoints.id
}

output "s3_prefix_list_id" {
  description = "The ID of the prefix list of Amazon S3 in this Region. A task pulls the layers of its image when its security group allows HTTPS to this prefix list."
  value       = aws_vpc_endpoint.s3.prefix_list_id
}
