# Centralized enrollement
resource "horizon_certificate" "example_centralized" {
  profile          = "EnrollmentProfile"
  key_type         = "rsa-2048"
  revoke_on_delete = true
  renew_before     = 30

  subject = [
    {
      element = "CN"
      type    = "CN"
      value   = "example.com"
    }
  ]
  sans = [
    {
      type  = "DNSNAME"
      value = ["example.com", "www.example.com"]
    }
  ]
  labels = [
    {
      label = "labelKey"
      value = "labelValue"
    }
  ]
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
  profile          = "EnrollmentProfile"
  revoke_on_delete = true

  labels = [
    {
      label = "labelKey"
      value = "labelValue"
    }
  ]
}