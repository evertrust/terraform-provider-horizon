resource "tls_private_key" "example" {
  algorithm = "RSA"
  rsa_bits  = 2048
}

resource "tls_cert_request" "example" {
  private_key_pem = tls_private_key.example.private_key_pem

  subject {
    common_name  = "example.com"
    organization = "ACME Examples, Inc"
  }
}

resource "horizon_certificate" "example" {
  csr = tls_cert_request.example.cert_request_pem
  profile   = "DefaultProfile"
  revoke_on_delete = false

  labels {
    label   = "label"
    value   = "example"
  }
}