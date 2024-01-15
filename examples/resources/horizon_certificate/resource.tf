# Centralized enrollement
resource "horizon_certificate" "example_centralized" {
  subject {
    element = "CN"
    type    = "CN"
    value   = "example.terraform.cn"
  }
  sans {
    element = "DNSNAME"
    type    = "DNSNAME"
    value   = "example.terraform.dnsname"
  }
  labels {
    label = "label"
    value = "example"
  }
  profile          = "Enrollment Profile"
  key_type         = "rsa-2048"
  revoke_on_delete = false
}

# Decentralized enrollment

resource "tls_private_key" "example_decentralized" {
  algorithm = "RSA"
  rsa_bits  = 2048
}

resource "tls_cert_request" "example_decentralized" {
  private_key_pem = tls_private_key.example_decentralized.private_key_pem

  subject {
    common_name  = "example_decentralized.com"
    organization = "ACME Examples, Inc"
  }
}

resource "horizon_certificate" "example_decentralized" {
  csr              = tls_cert_request.example_decentralized.cert_request_pem
  profile          = "DefaultProfile"
  revoke_on_delete = false

  labels {
    label = "label"
    value = "example"
  }
}