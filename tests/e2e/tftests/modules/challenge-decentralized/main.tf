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
  dns_names       = [var.cn]

  subject {
    common_name  = var.cn
    organization = "tf-test"
  }
}

# subject and sans are left out. On a challenge enrollment the provider sends
# the identity found in the CSR, because Horizon does not map the CSR.
resource "horizon_certificate" "test" {
  profile           = var.profile
  csr               = tls_cert_request.test.cert_request_pem
  request_challenge = true
}
