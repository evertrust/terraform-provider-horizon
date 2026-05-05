# Coverage for the optional metadata surface and the in-place metadata-update
# path of Update.
#
# Scope is currently limited by the Horizon test profile policy
# (resources/horizon_conf/2.{7,8}/db/profiles.json):
#
#   - contactEmailPolicy.editableByRequester = true     → tested
#   - ownerPolicy.editableByRequester        = false    → cannot be tested
#   - teamPolicy.editableByRequester         = false    → cannot be tested
#   - certificateTemplate has no `sans` array           → SANs rejected
#   - certificateTemplate has no `labelPolicies` array  → labels rejected
#
# The metadata module already supports owner/team/labels/sans variables, so
# expanding this file is a one-liner once the profile policy is loosened.
# See the // TODO comment in tests/e2e/resources/horizon_conf for the
# specific shape expected.

variable "endpoint" { type = string }
variable "username" { type = string }
variable "password" {
  type      = string
  sensitive = true
}
variable "centralized_profile" { type = string }

run "enroll_with_contact_email" {
  command = apply

  module {
    source = "./modules/metadata"
  }

  variables {
    endpoint      = var.endpoint
    username      = var.username
    password      = var.password
    profile       = var.centralized_profile
    cn            = "metadata-contact.tf-test.internal"
    contact_email = "alice@example.com"
  }

  assert {
    condition     = horizon_certificate.test.id != ""
    error_message = "enroll with contact_email must return an id"
  }
  assert {
    condition     = horizon_certificate.test.serial != ""
    error_message = "enroll with contact_email must return a serial"
  }
  assert {
    condition     = horizon_certificate.test.contact_email == "alice@example.com"
    error_message = "contact_email must be persisted in state"
  }
}

# Re-apply with identical config: must be a no-op (no spurious diff on
# contact_email / null optional fields).
run "metadata_no_drift" {
  command = apply

  module {
    source = "./modules/metadata"
  }

  variables {
    endpoint      = var.endpoint
    username      = var.username
    password      = var.password
    profile       = var.centralized_profile
    cn            = "metadata-contact.tf-test.internal"
    contact_email = "alice@example.com"
  }

  assert {
    condition     = horizon_certificate.test.id == run.enroll_with_contact_email.id
    error_message = "no-op apply must not change id"
  }
  assert {
    condition     = horizon_certificate.test.serial == run.enroll_with_contact_email.serial
    error_message = "no-op apply must not change serial"
  }
  assert {
    condition     = horizon_certificate.test.thumbprint == run.enroll_with_contact_email.thumbprint
    error_message = "no-op apply must not change thumbprint"
  }
}

# In-place metadata change: keep the same subject (no replace), flip
# contact_email. This is the canonical Update path with renewRequested=false
# and metadataChanged=true → WebRAUpdate runs, no renew.
run "metadata_in_place_update" {
  command = apply

  module {
    source = "./modules/metadata"
  }

  variables {
    endpoint      = var.endpoint
    username      = var.username
    password      = var.password
    profile       = var.centralized_profile
    cn            = "metadata-contact.tf-test.internal"
    contact_email = "bob@example.com"
  }

  assert {
    condition     = horizon_certificate.test.id == run.enroll_with_contact_email.id
    error_message = "metadata-only change must NOT replace the resource (id changed)"
  }
  assert {
    condition     = horizon_certificate.test.serial == run.enroll_with_contact_email.serial
    error_message = "metadata-only change must NOT trigger renewal (serial changed)"
  }
  assert {
    condition     = horizon_certificate.test.thumbprint == run.enroll_with_contact_email.thumbprint
    error_message = "metadata-only change must NOT trigger renewal (thumbprint changed)"
  }
  assert {
    condition     = horizon_certificate.test.contact_email == "bob@example.com"
    error_message = "contact_email must reflect the new value after in-place update"
  }
}

# Clear the optional contact_email: subject stays the same → still in-place
# update. State must reflect the cleared value with no perpetual drift.
run "metadata_cleared" {
  command = apply

  module {
    source = "./modules/metadata"
  }

  variables {
    endpoint      = var.endpoint
    username      = var.username
    password      = var.password
    profile       = var.centralized_profile
    cn            = "metadata-contact.tf-test.internal"
    contact_email = null
  }

  assert {
    condition     = horizon_certificate.test.id == run.enroll_with_contact_email.id
    error_message = "clearing optional metadata must not replace the resource"
  }
  assert {
    condition     = horizon_certificate.test.contact_email == null
    error_message = "contact_email must be null after being cleared"
  }
}

# revoke_on_delete=true: the resource is destroyed at the end of the file
# (terraform test cleans up). This run only verifies that the enrollment
# accepts the flag; the actual revoke call happens during cleanup and would
# surface as a Delete error in the run output if it failed.
run "enroll_with_revoke_on_delete" {
  command = apply

  module {
    source = "./modules/metadata"
  }

  variables {
    endpoint         = var.endpoint
    username         = var.username
    password         = var.password
    profile          = var.centralized_profile
    cn               = "metadata-revoke.tf-test.internal"
    revoke_on_delete = true
  }

  assert {
    condition     = horizon_certificate.test.id != ""
    error_message = "enroll with revoke_on_delete=true must succeed"
  }
}
