# Centralized enrollment
#
# renew_before = 30 days: once a plan runs inside the renewal window, the
# provider performs an in-place WebRA renew. Terraform sees an in-place
# update; the resource address stays the same and computed fields
# (serial, thumbprint, ...) are refreshed from the renewed certificate.
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
# Terraform side. Inside the renew_before window the provider issues an
# in-place WebRA renew, forwarding the current CSR to Horizon. Reusing the
# same CSR keeps the same key; if you want a fresh key on renewal,
# regenerate the CSR-producing resource (e.g. taint tls_private_key) so a
# new CSR reaches the renew call.
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

# Enrollment with a WebRA challenge (Horizon 2.11+)
#
# On a profile in Challenge authorization mode, Horizon authorizes the
# enrollment with the one-time challenge, so the provider credentials need no
# enroll permission on the profile. The provider still uses them to read, renew
# and revoke the certificate. Horizon encrypts the PKCS#12 with the challenge,
# which the provider exposes as `password`.
variable "webra_challenge" {
  type      = string
  sensitive = true
}

resource "horizon_certificate" "example_challenge" {
  profile   = "ChallengeProfile"
  key_type  = "rsa-2048"
  challenge = var.webra_challenge

  # Horizon uses subject and sans when the certificate template of the profile
  # is empty. With a defined template, it keeps the identity that was set when
  # the challenge was issued.
  subject = [
    {
      element = "cn.1"
      type    = "CN"
      value   = "challenge.example.com"
    }
  ]
  sans = [
    {
      type  = "DNSNAME"
      value = ["challenge.example.com"]
    }
  ]
}

# Same profile when nobody gave you a challenge. The provider requests one with
# its own credentials, which need the enroll and approve permissions, and
# consumes it in the same apply.
resource "horizon_certificate" "example_request_challenge" {
  profile           = "ChallengeProfile"
  key_type          = "rsa-2048"
  request_challenge = true

  subject = [
    {
      element = "cn.1"
      type    = "CN"
      value   = "requested-challenge.example.com"
    }
  ]
}
