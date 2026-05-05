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

# Enroll a fresh certificate so we have a real PEM to feed the data source.
resource "horizon_certificate" "test" {
  profile  = var.profile
  key_type = "rsa-2048"

  subject = [
    {
      element = "cn.1"
      type    = "CN"
      value   = var.cn
    }
  ]
}

data "horizon_certificate_trust_chain" "default_order" {
  certificate_pem = horizon_certificate.test.certificate
}

data "horizon_certificate_trust_chain" "leaf_to_root" {
  certificate_pem = horizon_certificate.test.certificate
  order           = "leaf_to_root"
}

data "horizon_certificate_trust_chain" "root_to_leaf" {
  certificate_pem = horizon_certificate.test.certificate
  order           = "root_to_leaf"
}

data "horizon_certificate_trust_chain" "issuer_leaf_to_root" {
  certificate_pem = horizon_certificate.test.certificate
  order           = "issuer_leaf_to_root"
}

data "horizon_certificate_trust_chain" "issuer_root_to_leaf" {
  certificate_pem = horizon_certificate.test.certificate
  order           = "issuer_root_to_leaf"
}
