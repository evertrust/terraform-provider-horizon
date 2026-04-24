# Centralized enrollment
#
# renew_before = 30 days: once a plan runs inside the renewal window, the
# provider performs an in-place WebRA renew (Terraform sees an update, not a
# destroy/create).
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
  wait_for_third_parties = [
    "my-aws-connector"
  ]
}

# Centralized enrollment with write-only PKCS12 and password
#
# The generated PKCS12 bundle and its password are not persisted to Terraform
# state (useful to keep sensitive material out of state files).
resource "horizon_certificate" "example_centralized_write_only" {
  profile             = "EnrollmentProfile"
  key_type            = "rsa-2048"
  pkcs12_write_only   = true
  password_write_only = true

  subject = [
    {
      element = "CN"
      type    = "CN"
      value   = "write-only.example.com"
    }
  ]
}

# Decentralized enrollment
#
# When `csr` is set, enrollment is decentralized: the private key stays on the
# Terraform side. Renewal also happens there — inside the renew_before window,
# the provider plans a destroy/create (RequiresReplace) rather than a WebRA
# renew, since a renew with the same CSR would reuse the key.
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
  renew_before     = 30

  labels = [
    {
      label = "labelKey"
      value = "labelValue"
    }
  ]
}
