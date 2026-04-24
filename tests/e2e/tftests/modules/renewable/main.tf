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

resource "horizon_certificate" "test" {
  profile      = var.profile
  key_type     = "rsa-2048"
  renew_before = var.renew_before_days

  subject = [
    {
      element = "cn.1"
      type    = "CN"
      value   = var.cn
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

output "not_before" {
  value = horizon_certificate.test.not_before
}

output "not_after" {
  value = horizon_certificate.test.not_after
}
