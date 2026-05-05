# Out-of-window scenarios: when a certificate is comfortably ahead of its
# renew_before threshold, neither Read nor ModifyPlan must produce a diff.
#
# CA issues ~1-year certs; renew_before_days = 10 puts the renewal window
# far in the future, so a second apply with identical config must be a
# perfect no-op (same id / serial / thumbprint).

variable "endpoint" { type = string }
variable "username" { type = string }
variable "password" {
  type      = string
  sensitive = true
}
variable "centralized_profile" { type = string }
variable "decentralized_profile" { type = string }

run "enroll_centralized_out_of_window" {
  command = apply

  module {
    source = "./modules/renewable"
  }

  variables {
    endpoint          = var.endpoint
    username          = var.username
    password          = var.password
    profile           = var.centralized_profile
    cn                = "no-drift-centralized.tf-test.internal"
    renew_before_days = 10
  }

  assert {
    condition     = horizon_certificate.test.id != ""
    error_message = "initial centralized enrollment must return an id"
  }
  assert {
    condition     = horizon_certificate.test.not_after > 0
    error_message = "initial centralized enrollment must return not_after"
  }
}

run "no_drift_centralized" {
  command = apply

  module {
    source = "./modules/renewable"
  }

  variables {
    endpoint          = var.endpoint
    username          = var.username
    password          = var.password
    profile           = var.centralized_profile
    cn                = "no-drift-centralized.tf-test.internal"
    renew_before_days = 10
  }

  # A second apply with the same config and a cert far from its renew_before
  # window must not trigger a renewal. id / serial / thumbprint must match
  # the initial enrollment exactly.
  assert {
    condition     = horizon_certificate.test.id == run.enroll_centralized_out_of_window.id
    error_message = "centralized: id changed outside the renew_before window — ModifyPlan over-triggered renewal"
  }
  assert {
    condition     = horizon_certificate.test.serial == run.enroll_centralized_out_of_window.serial
    error_message = "centralized: serial changed outside the renew_before window — ModifyPlan over-triggered renewal"
  }
  assert {
    condition     = horizon_certificate.test.thumbprint == run.enroll_centralized_out_of_window.thumbprint
    error_message = "centralized: thumbprint changed outside the renew_before window — ModifyPlan over-triggered renewal"
  }
  assert {
    condition     = horizon_certificate.test.not_after == run.enroll_centralized_out_of_window.not_after
    error_message = "centralized: not_after changed outside the renew_before window"
  }
}

run "enroll_decentralized_out_of_window" {
  command = apply

  module {
    source = "./modules/renewable-decentralized"
  }

  variables {
    endpoint          = var.endpoint
    username          = var.username
    password          = var.password
    profile           = var.decentralized_profile
    cn                = "no-drift-decentralized.tf-test.internal"
    renew_before_days = 10
  }

  assert {
    condition     = horizon_certificate.test.id != ""
    error_message = "initial decentralized enrollment must return an id"
  }
  assert {
    condition     = horizon_certificate.test.not_after > 0
    error_message = "initial decentralized enrollment must return not_after"
  }
}

run "no_drift_decentralized" {
  command = apply

  module {
    source = "./modules/renewable-decentralized"
  }

  variables {
    endpoint          = var.endpoint
    username          = var.username
    password          = var.password
    profile           = var.decentralized_profile
    cn                = "no-drift-decentralized.tf-test.internal"
    renew_before_days = 10
  }

  # Same expectation as the centralized case: a CSR-backed cert outside its
  # renew window must not be touched. If ModifyPlan flipped renewal_trigger
  # Unknown here, Update would issue a WebRA renew and serial/thumbprint
  # would differ.
  assert {
    condition     = horizon_certificate.test.id == run.enroll_decentralized_out_of_window.id
    error_message = "decentralized: id changed outside the renew_before window — ModifyPlan over-triggered renewal"
  }
  assert {
    condition     = horizon_certificate.test.serial == run.enroll_decentralized_out_of_window.serial
    error_message = "decentralized: serial changed outside the renew_before window — ModifyPlan over-triggered renewal"
  }
  assert {
    condition     = horizon_certificate.test.thumbprint == run.enroll_decentralized_out_of_window.thumbprint
    error_message = "decentralized: thumbprint changed outside the renew_before window — ModifyPlan over-triggered renewal"
  }
}
