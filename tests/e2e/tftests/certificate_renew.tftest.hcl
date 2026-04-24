# Real WebRA renewal flow:
#
#   1. Read refreshes computed attributes from Horizon without side effects.
#   2. ModifyPlan detects that the certificate is within the renew_before
#      window (not_after - renew_before days <= now) and:
#        - in centralized mode, marks renewal_trigger Unknown so Terraform
#          plans an in-place Update that calls the WebRA renew endpoint.
#        - in decentralized mode, adds renewal_trigger to RequiresReplace so
#          Terraform plans a destroy/create (a WebRA renew with the same CSR
#          would just reuse the key material, which is rarely desirable).
#
# The CA issues ~1 year certs; renew_before_days = 400 guarantees the
# renewal window is already open immediately after enroll, so the second
# run block always triggers a real renewal.

variable "endpoint" { type = string }
variable "username" { type = string }
variable "password" {
  type      = string
  sensitive = true
}
variable "centralized_profile" { type = string }
variable "decentralized_profile" { type = string }

run "enroll_centralized" {
  command = apply

  module {
    source = "./modules/renewable"
  }

  variables {
    endpoint          = var.endpoint
    username          = var.username
    password          = var.password
    profile           = var.centralized_profile
    cn                = "renew-centralized.tf-test.internal"
    renew_before_days = 400
  }

  assert {
    condition     = horizon_certificate.test.id != ""
    error_message = "initial centralized enrollment must return an id"
  }
  assert {
    condition     = horizon_certificate.test.serial != ""
    error_message = "initial centralized enrollment must return a serial"
  }
  assert {
    condition     = horizon_certificate.test.thumbprint != ""
    error_message = "initial centralized enrollment must return a thumbprint"
  }
  assert {
    condition     = horizon_certificate.test.not_after > 0
    error_message = "initial centralized enrollment must return not_after"
  }
}

run "renew_centralized_in_place" {
  command = apply

  module {
    source = "./modules/renewable"
  }

  variables {
    endpoint          = var.endpoint
    username          = var.username
    password          = var.password
    profile           = var.centralized_profile
    cn                = "renew-centralized.tf-test.internal"
    renew_before_days = 400
  }

  # Centralized: ModifyPlan marks renewal_trigger as Unknown inside the
  # window, so this apply runs Update (not Create) and calls WebRA renew.
  # Horizon returns a new serial/thumbprint and may or may not preserve the
  # internal id depending on its version, so we don't assert on id.
  assert {
    condition     = horizon_certificate.test.serial != run.enroll_centralized.serial
    error_message = "centralized: serial must change after a successful WebRA renew"
  }
  assert {
    condition     = horizon_certificate.test.thumbprint != run.enroll_centralized.thumbprint
    error_message = "centralized: thumbprint must change after a successful WebRA renew"
  }
  assert {
    condition     = horizon_certificate.test.not_after > 0
    error_message = "centralized: renewed cert must carry a non-zero not_after"
  }
}

run "enroll_decentralized" {
  command = apply

  module {
    source = "./modules/renewable-decentralized"
  }

  variables {
    endpoint          = var.endpoint
    username          = var.username
    password          = var.password
    profile           = var.decentralized_profile
    cn                = "renew-decentralized.tf-test.internal"
    renew_before_days = 400
  }

  assert {
    condition     = horizon_certificate.test.id != ""
    error_message = "initial decentralized enrollment must return an id"
  }
  assert {
    condition     = horizon_certificate.test.serial != ""
    error_message = "initial decentralized enrollment must return a serial"
  }
  assert {
    condition     = horizon_certificate.test.thumbprint != ""
    error_message = "initial decentralized enrollment must return a thumbprint"
  }
  assert {
    condition     = horizon_certificate.test.not_after > 0
    error_message = "initial decentralized enrollment must return not_after"
  }
}

run "renew_decentralized_replace" {
  command = apply

  module {
    source = "./modules/renewable-decentralized"
  }

  variables {
    endpoint          = var.endpoint
    username          = var.username
    password          = var.password
    profile           = var.decentralized_profile
    cn                = "renew-decentralized.tf-test.internal"
    renew_before_days = 400
  }

  # Decentralized: ModifyPlan appends renewal_trigger to RequiresReplace, so
  # this apply runs Delete + Create. The enrollment produces a fresh cert in
  # Horizon (new id, new serial, new thumbprint).
  assert {
    condition     = horizon_certificate.test.id != run.enroll_decentralized.id
    error_message = "decentralized: id must change — renewal uses destroy/create when a CSR is provided"
  }
  assert {
    condition     = horizon_certificate.test.serial != run.enroll_decentralized.serial
    error_message = "decentralized: serial must change after a renew-triggered re-enrollment"
  }
  assert {
    condition     = horizon_certificate.test.thumbprint != run.enroll_decentralized.thumbprint
    error_message = "decentralized: thumbprint must change after a renew-triggered re-enrollment"
  }
  assert {
    condition     = horizon_certificate.test.not_after > 0
    error_message = "decentralized: re-enrolled cert must carry a non-zero not_after"
  }
}
