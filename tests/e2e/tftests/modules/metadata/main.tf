terraform {
  required_providers {
    horizon = {
      source = "registry.terraform.io/evertrust/horizon"
    }
  }
}

provider "horizon" {
  endpoint = var.endpoint
  username = var.username
  password = var.password
}

# sans / labels are SetNestedAttribute (not blocks), so they must be assigned
# as collection values. Empty collection == omit.
resource "horizon_certificate" "test" {
  profile          = var.profile
  key_type         = "rsa-2048"
  owner            = var.owner
  team             = var.team
  contact_email    = var.contact_email
  revoke_on_delete = var.revoke_on_delete

  subject = [
    {
      element = "cn.1"
      type    = "CN"
      value   = var.cn
    }
  ]

  sans = length(var.sans_dns) > 0 ? [
    {
      type  = "DNSNAME"
      value = var.sans_dns
    }
  ] : null

  labels = length(var.labels) > 0 ? [
    for k, v in var.labels : {
      label = k
      value = v
    }
  ] : null
}

output "id" {
  value = horizon_certificate.test.id
}

output "serial" {
  value = horizon_certificate.test.serial
}

output "thumbprint" {
  value = horizon_certificate.test.thumbprint
}

output "dn" {
  value = horizon_certificate.test.dn
}
