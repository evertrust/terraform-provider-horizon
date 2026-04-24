# Current renewal behavior — not a real WebRA renew, but a forced
# re-enrollment via the Read → RemoveResource → Create pattern:
#
#   1. Read pulls the cert from Horizon, computes
#      renewal_date = not_after - renew_before days, and if now is past it
#      the resource is removed from the Terraform state.
#   2. Terraform plans a Create on the next run because the resource
#      disappeared from state.
#   3. Create enrolls a fresh certificate via NewEnrollRequest.
#
# The CA issues ~1 year certs; renew_before_days = 400 guarantees the
# renewal window is already open immediately after enroll, so the second
# run block always triggers a brand-new enrollment.

variable "endpoint" { type = string }
variable "username" { type = string }
variable "password" {
  type      = string
  sensitive = true
}
variable "centralized_profile" { type = string }

run "enroll" {
  command = apply

  module {
    source = "./modules/renewable"
  }

  variables {
    endpoint          = var.endpoint
    username          = var.username
    password          = var.password
    profile           = var.centralized_profile
    cn                = "renew.tf-test.internal"
    renew_before_days = 400
  }

  assert {
    condition     = horizon_certificate.test.id != ""
    error_message = "initial enrollment must return an id"
  }
  assert {
    condition     = horizon_certificate.test.serial != ""
    error_message = "initial enrollment must return a serial"
  }
  assert {
    condition     = horizon_certificate.test.thumbprint != ""
    error_message = "initial enrollment must return a thumbprint"
  }
  assert {
    condition     = horizon_certificate.test.not_after > 0
    error_message = "initial enrollment must return not_after"
  }
}

run "renew_via_recreate" {
  command = apply

  module {
    source = "./modules/renewable"
  }

  variables {
    endpoint          = var.endpoint
    username          = var.username
    password          = var.password
    profile           = var.centralized_profile
    cn                = "renew.tf-test.internal"
    renew_before_days = 400
  }

  # Because Read removed the resource from state, this apply calls Create
  # again. The cert re-issued by NewEnrollRequest must be a fresh one —
  # new id, new serial, new thumbprint — confirming the workaround is
  # actually happening rather than Terraform silently reusing the state.
  assert {
    condition     = horizon_certificate.test.id != run.enroll.id
    error_message = "id must change — remove-from-state should force a fresh Create"
  }
  assert {
    condition     = horizon_certificate.test.serial != run.enroll.serial
    error_message = "serial must change when Read removes the resource and Create runs a new enrollment"
  }
  assert {
    condition     = horizon_certificate.test.thumbprint != run.enroll.thumbprint
    error_message = "thumbprint must change between the two enrollments"
  }
  assert {
    condition     = horizon_certificate.test.not_after > 0
    error_message = "renewed cert must carry not_after"
  }
}
