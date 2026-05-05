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
  profile             = var.profile
  key_type            = "rsa-2048"
  pkcs12_write_only   = var.pkcs12_write_only
  password_write_only = var.password_write_only

  subject = [
    {
      element = "cn.1"
      type    = "CN"
      value   = var.cn
    }
  ]
}
