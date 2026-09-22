# horizon_retrieve_centralized_pkcs12 against a real Horizon. The certificate
# is enrolled with both write-only flags, so the ephemeral resource is the only
# way to get the PKCS#12 and its password: they must never reach the state.
#
# terraform test cannot assert on ephemeral resources, so the expectations on
# the retrieved material are a check block inside ./modules/retrieve_pkcs12; a
# failing check fails the run. expected_source tells that check which Horizon
# request must serve the material. It has to follow the certificate across a
# renewal: a renewed certificate has a new id, and its PKCS#12 lives in the
# renew request, not in the original enroll request.

variable "endpoint" { type = string }
variable "username" { type = string }
variable "password" {
  type      = string
  sensitive = true
}
variable "centralized_profile" { type = string }

run "retrieve_after_enroll" {
  command = apply

  module {
    source = "./modules/retrieve_pkcs12"
  }

  variables {
    endpoint        = var.endpoint
    username        = var.username
    password        = var.password
    profile         = var.centralized_profile
    cn              = "retrieve-pkcs12.tf-test.internal"
    expected_source = "enroll_request"
  }

  assert {
    condition     = horizon_certificate.test.pkcs12 == null && horizon_certificate.test.password == null
    error_message = "write-only certificate must not persist pkcs12/password to state"
  }
}

# renew_before_days = 400 opens the renewal window, so this apply renews the
# certificate in place and the ephemeral resource is opened with the new id.
run "retrieve_after_renew" {
  command = apply

  module {
    source = "./modules/retrieve_pkcs12"
  }

  variables {
    endpoint          = var.endpoint
    username          = var.username
    password          = var.password
    profile           = var.centralized_profile
    cn                = "retrieve-pkcs12.tf-test.internal"
    renew_before_days = 400
    expected_source   = "renew_request"
  }

  assert {
    condition     = horizon_certificate.test.serial != run.retrieve_after_enroll.serial
    error_message = "certificate must have been renewed"
  }
}

# skip_escrow_check = true restricts the lookup to existing enroll/renew
# requests (no recovery). The renewed certificate from the previous run still
# has its renew request, so material must be found. renew_before_days is unset
# so this apply does not renew again.
run "retrieve_with_skip_escrow_check" {
  command = apply

  module {
    source = "./modules/retrieve_pkcs12"
  }

  variables {
    endpoint          = var.endpoint
    username          = var.username
    password          = var.password
    profile           = var.centralized_profile
    cn                = "retrieve-pkcs12.tf-test.internal"
    skip_escrow_check = true
    expected_source   = "renew_request"
  }

  assert {
    condition     = horizon_certificate.test.serial == run.retrieve_after_renew.serial
    error_message = "certificate must not be renewed again"
  }
}
