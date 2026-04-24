terraform {
  required_providers {
    horizon = {
      source = "registry.terraform.io/evertrust/horizon"
    }
    tls = {
      source  = "hashicorp/tls"
      version = "~> 4.0"
    }
  }
}

provider "horizon" {
  endpoint = var.endpoint
  username = var.username
  password = var.password
}

resource "tls_private_key" "test" {
  algorithm = "RSA"
  rsa_bits  = 2048
}

resource "tls_cert_request" "test" {
  private_key_pem = tls_private_key.test.private_key_pem

  subject {
    common_name = var.cn
  }
}

resource "horizon_certificate" "test" {
  profile      = var.profile
  csr          = tls_cert_request.test.cert_request_pem
  renew_before = var.renew_before_days
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
