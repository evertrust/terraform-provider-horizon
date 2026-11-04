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

# Centralized enrollment on a WebRA profile in Challenge authorization mode.
# Set var.challenge to use a challenge issued beforehand, or
# var.request_challenge to let the provider request one.
resource "horizon_certificate" "test" {
  profile           = var.profile
  key_type          = "rsa-2048"
  challenge         = var.challenge
  request_challenge = var.request_challenge
  renew_before      = var.renew_before_days
  contact_email     = var.contact_email

  subject = [
    {
      element = "cn.1"
      type    = "CN"
      value   = var.cn
    }
  ]

  sans = [
    {
      type  = "DNSNAME"
      value = [var.cn]
    }
  ]
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
