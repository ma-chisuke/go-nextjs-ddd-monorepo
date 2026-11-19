# The network of one environment (TPL-0015): a VPC with three kinds of subnet
# in each Availability Zone.
#
#   public     the load balancer of the web front end
#   private    the tasks of the contexts and of the web front end
#   database   the Aurora cluster
#
# Only what sits in a public subnet can be reached from the internet, so the
# APIs of the contexts stay private by where they run (TPL-0014).
#
# By default nothing in the private subnets can reach the internet either.
# The tasks reach the AWS services that they call through VPC endpoints (AWS
# PrivateLink), so their traffic stays inside the VPC, and their security
# groups allow no address outside it. A NAT gateway is there for the team
# whose context calls a service outside AWS (var.nat_gateways).

locals {
  zones = { for index, zone in var.availability_zones : zone => index }

  # The zones whose public subnet holds a NAT gateway.
  nat_zones = {
    none     = {}
    single   = { (var.availability_zones[0]) = 0 }
    per-zone = local.zones
  }[var.nat_gateways]

  # The zones whose private subnet holds a network interface of each
  # interface endpoint.
  endpoint_zones = var.interface_endpoints_in_every_zone ? var.availability_zones : [var.availability_zones[0]]
}

data "aws_region" "current" {}

resource "aws_vpc" "this" {
  cidr_block           = var.cidr_block
  enable_dns_support   = true
  enable_dns_hostnames = true

  tags = {
    Name = var.environment
  }
}

# The default security group of a VPC allows all traffic between its members.
# Managing it without rules closes it, so a resource that names no security
# group can talk to nothing.
resource "aws_default_security_group" "this" {
  vpc_id = aws_vpc.this.id

  tags = {
    Name = "${var.environment}-default-unused"
  }
}

# Subnets -------------------------------------------------------------------

# The address range is cut into 16 parts: the public subnets take the parts
# from 0, the private ones from 4, and the database ones from 8, one for each
# of up to four zones.
resource "aws_subnet" "public" {
  for_each = local.zones

  vpc_id            = aws_vpc.this.id
  availability_zone = each.key
  cidr_block        = cidrsubnet(var.cidr_block, 4, each.value)

  # A load balancer and a NAT gateway get their public addresses themselves.
  map_public_ip_on_launch = false

  tags = {
    Name = "${var.environment}-public-${each.key}"
  }
}

resource "aws_subnet" "private" {
  for_each = local.zones

  vpc_id            = aws_vpc.this.id
  availability_zone = each.key
  cidr_block        = cidrsubnet(var.cidr_block, 4, 4 + each.value)

  tags = {
    Name = "${var.environment}-private-${each.key}"
  }
}

resource "aws_subnet" "database" {
  for_each = local.zones

  vpc_id            = aws_vpc.this.id
  availability_zone = each.key
  cidr_block        = cidrsubnet(var.cidr_block, 4, 8 + each.value)

  tags = {
    Name = "${var.environment}-database-${each.key}"
  }
}

# Routes --------------------------------------------------------------------

resource "aws_internet_gateway" "this" {
  vpc_id = aws_vpc.this.id

  tags = {
    Name = var.environment
  }
}

resource "aws_route_table" "public" {
  vpc_id = aws_vpc.this.id

  tags = {
    Name = "${var.environment}-public"
  }
}

resource "aws_route" "public_internet" {
  route_table_id         = aws_route_table.public.id
  destination_cidr_block = "0.0.0.0/0"
  gateway_id             = aws_internet_gateway.this.id
}

resource "aws_route_table_association" "public" {
  for_each = local.zones

  subnet_id      = aws_subnet.public[each.key].id
  route_table_id = aws_route_table.public.id
}

# One route table for each zone, so that each zone can use the NAT gateway of
# its own zone when there is one. Without a NAT gateway, a private subnet has
# the local route and the route to Amazon S3 alone.
resource "aws_route_table" "private" {
  for_each = local.zones

  vpc_id = aws_vpc.this.id

  tags = {
    Name = "${var.environment}-private-${each.key}"
  }
}

resource "aws_route_table_association" "private" {
  for_each = local.zones

  subnet_id      = aws_subnet.private[each.key].id
  route_table_id = aws_route_table.private[each.key].id
}

# The database subnets have the local route alone.
resource "aws_route_table" "database" {
  vpc_id = aws_vpc.this.id

  tags = {
    Name = "${var.environment}-database"
  }
}

resource "aws_route_table_association" "database" {
  for_each = local.zones

  subnet_id      = aws_subnet.database[each.key].id
  route_table_id = aws_route_table.database.id
}

# Endpoints -----------------------------------------------------------------

# The layers of a container image come from Amazon S3, through a gateway
# endpoint, which costs nothing. A security group names Amazon S3 by the
# prefix list of this endpoint.
resource "aws_vpc_endpoint" "s3" {
  vpc_id            = aws_vpc.this.id
  service_name      = "com.amazonaws.${data.aws_region.current.region}.s3"
  vpc_endpoint_type = "Gateway"
  route_table_ids   = [for table in aws_route_table.private : table.id]

  tags = {
    Name = "${var.environment}-s3"
  }
}

# The interface endpoints accept HTTPS from inside the VPC. A task reaches
# them when its own security group allows HTTPS to this group.
resource "aws_security_group" "endpoints" {
  name        = "${var.environment}-endpoints"
  description = "The interface endpoints of the AWS services."
  vpc_id      = aws_vpc.this.id

  tags = {
    Name = "${var.environment}-endpoints"
  }
}

resource "aws_vpc_security_group_ingress_rule" "endpoints_https" {
  security_group_id = aws_security_group.endpoints.id
  description       = "HTTPS from inside the VPC."
  cidr_ipv4         = aws_vpc.this.cidr_block
  from_port         = 443
  to_port           = 443
  ip_protocol       = "tcp"
}

# With private DNS, the usual name of a service, such as
# logs.<region>.amazonaws.com, resolves to the endpoint inside the VPC, so
# the SDKs and the ECS agent need no setting.
resource "aws_vpc_endpoint" "interface" {
  for_each = var.interface_endpoints

  vpc_id              = aws_vpc.this.id
  service_name        = "com.amazonaws.${data.aws_region.current.region}.${each.key}"
  vpc_endpoint_type   = "Interface"
  subnet_ids          = [for zone in local.endpoint_zones : aws_subnet.private[zone].id]
  security_group_ids  = [aws_security_group.endpoints.id]
  private_dns_enabled = true

  tags = {
    Name = "${var.environment}-${each.key}"
  }
}

# NAT gateways (optional) ---------------------------------------------------

resource "aws_eip" "nat" {
  for_each = local.nat_zones

  domain = "vpc"

  tags = {
    Name = "${var.environment}-nat-${each.key}"
  }
}

resource "aws_nat_gateway" "this" {
  for_each = local.nat_zones

  allocation_id = aws_eip.nat[each.key].id
  subnet_id     = aws_subnet.public[each.key].id

  tags = {
    Name = "${var.environment}-${each.key}"
  }

  # A NAT gateway needs the internet gateway of its VPC.
  depends_on = [aws_internet_gateway.this]
}

resource "aws_route" "private_internet" {
  for_each = var.nat_gateways == "none" ? {} : local.zones

  route_table_id         = aws_route_table.private[each.key].id
  destination_cidr_block = "0.0.0.0/0"
  nat_gateway_id         = aws_nat_gateway.this[var.nat_gateways == "single" ? var.availability_zones[0] : each.key].id
}

# Flow logs -----------------------------------------------------------------

# What the network accepted and rejected, for the day someone asks who talked
# to whom.
resource "aws_cloudwatch_log_group" "flow" {
  name              = "/${var.environment}/vpc/flow"
  retention_in_days = var.flow_log_retention_days
}

data "aws_iam_policy_document" "flow_assume" {
  statement {
    actions = ["sts:AssumeRole"]

    principals {
      type        = "Service"
      identifiers = ["vpc-flow-logs.amazonaws.com"]
    }
  }
}

data "aws_iam_policy_document" "flow_write" {
  statement {
    actions = [
      "logs:CreateLogStream",
      "logs:PutLogEvents",
      "logs:DescribeLogStreams",
    ]
    resources = ["${aws_cloudwatch_log_group.flow.arn}:*"]
  }
}

resource "aws_iam_role" "flow" {
  name               = "${var.environment}-vpc-flow-logs"
  assume_role_policy = data.aws_iam_policy_document.flow_assume.json
}

resource "aws_iam_role_policy" "flow" {
  name   = "write-flow-logs"
  role   = aws_iam_role.flow.id
  policy = data.aws_iam_policy_document.flow_write.json
}

resource "aws_flow_log" "this" {
  vpc_id               = aws_vpc.this.id
  traffic_type         = "ALL"
  log_destination_type = "cloud-watch-logs"
  log_destination      = aws_cloudwatch_log_group.flow.arn
  iam_role_arn         = aws_iam_role.flow.arn

  tags = {
    Name = var.environment
  }
}
