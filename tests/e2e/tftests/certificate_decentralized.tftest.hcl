# Decentralized enrollment scenarios. The child module generates an RSA-2048
# private key + CSR via the tls provider, then enrolls with that CSR.

variable "endpoint" { type = string }
variable "username" { type = string }
variable "password" {
  type      = string
  sensitive = true
}
variable "decentralized_profile" { type = string }

run "decentralized_basic" {
  command = apply

  module {
    source = "./modules/decentralized"
  }

  variables {
    endpoint = var.endpoint
    username = var.username
    password = var.password
    profile  = var.decentralized_profile
    cn       = "decentralized-basic.tf-test.internal"
  }

  assert {
    condition     = horizon_certificate.test.id != ""
    error_message = "certificate id must be set"
  }
  assert {
    condition     = horizon_certificate.test.thumbprint != ""
    error_message = "thumbprint must be set"
  }
  assert {
    condition     = horizon_certificate.test.certificate != null && horizon_certificate.test.certificate != ""
    error_message = "certificate PEM must be set"
  }
  assert {
    condition     = horizon_certificate.test.pkcs12 == null
    error_message = "pkcs12 must not be returned in decentralized mode"
  }
  assert {
    condition     = horizon_certificate.test.password == null
    error_message = "password must not be returned in decentralized mode"
  }
}

run "decentralized_metadata" {
  command = apply

  module {
    source = "./modules/decentralized"
  }

  variables {
    endpoint = var.endpoint
    username = var.username
    password = var.password
    profile  = var.decentralized_profile
    cn       = "decentralized-metadata.tf-test.internal"
  }

  assert {
    condition     = horizon_certificate.test.dn != ""
    error_message = "dn must be set"
  }
  assert {
    condition     = horizon_certificate.test.serial != ""
    error_message = "serial must be set"
  }
  assert {
    condition     = horizon_certificate.test.issuer != ""
    error_message = "issuer must be set"
  }
  assert {
    condition     = horizon_certificate.test.not_before != 0
    error_message = "not_before must be set"
  }
  assert {
    condition     = horizon_certificate.test.not_after != 0
    error_message = "not_after must be set"
  }
  assert {
    condition     = horizon_certificate.test.key_type != ""
    error_message = "key_type must be set"
  }
  assert {
    condition     = horizon_certificate.test.signing_algorithm != ""
    error_message = "signing_algorithm must be set"
  }
}
