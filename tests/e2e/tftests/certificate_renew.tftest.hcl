# Real WebRA renewal flow (in-place for both centralized and decentralized):
#
#   1. Read refreshes computed attributes from Horizon without side effects.
#   2. ModifyPlan detects that the certificate is within the renew_before
#      window (not_after - renew_before days <= now) and marks
#      renewal_trigger Unknown so Terraform plans an in-place Update.
#   3. Update calls the WebRA renew endpoint, forwarding the CSR for
#      decentralized enrollments. Key rotation in decentralized mode is the
#      user's responsibility — they regenerate the CSR-producing resource
#      (e.g. tls_private_key) when they want a fresh key on renewal.
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
  # The test CA stamps not_after with second-level resolution and the two
  # enrollments often happen within the same second, so we can only assert
  # that the renewed cert wasn't issued strictly earlier than the original.
  # The actual proof of renewal is the serial/thumbprint rotation above.
  assert {
    condition     = horizon_certificate.test.not_after >= run.enroll_centralized.not_after
    error_message = "centralized: renewed cert must not regress not_after"
  }
  # Same logical certificate identity: subject DN and issuer must match
  # the original. A diverging DN/issuer would mean we re-enrolled
  # somewhere else rather than renewed.
  assert {
    condition     = horizon_certificate.test.dn == run.enroll_centralized.dn
    error_message = "centralized: dn must be preserved across renewal"
  }
  assert {
    condition     = horizon_certificate.test.issuer == run.enroll_centralized.issuer
    error_message = "centralized: issuer must be preserved across renewal"
  }
}

# Mixed-update path: renewal AND metadata change happen in the same apply.
# Update should call WebRA renew first, then WebRA update for the metadata
# delta (the conditional in Update.go fires because metadataChanged() is
# true). The cert must end up with both the renewed material AND the new
# metadata.
run "renew_centralized_with_metadata_update" {
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
    contact_email     = "renewed-owner@tf-test.internal"
  }

  # Renewal half: serial / thumbprint must rotate again.
  assert {
    condition     = horizon_certificate.test.serial != run.renew_centralized_in_place.serial
    error_message = "mixed: serial must change again on a second renewal cycle"
  }
  assert {
    condition     = horizon_certificate.test.thumbprint != run.renew_centralized_in_place.thumbprint
    error_message = "mixed: thumbprint must change again on a second renewal cycle"
  }
  assert {
    condition     = horizon_certificate.test.not_after >= run.renew_centralized_in_place.not_after
    error_message = "mixed: not_after must not regress after renewal"
  }
  # Metadata half: contact_email landed in state.
  assert {
    condition     = horizon_certificate.test.contact_email == "renewed-owner@tf-test.internal"
    error_message = "mixed: contact_email must reflect the updated value after a renew+update apply"
  }
  # Same logical identity preserved.
  assert {
    condition     = horizon_certificate.test.dn == run.enroll_centralized.dn
    error_message = "mixed: dn must still match the original enrollment"
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

run "renew_decentralized_in_place" {
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

  # Decentralized: ModifyPlan now flips renewal_trigger Unknown (no replace),
  # so Update runs and calls WebRA renew with the existing CSR. Horizon
  # returns a new serial/thumbprint and may or may not preserve the
  # internal id depending on its version, so we don't assert on id.
  assert {
    condition     = horizon_certificate.test.serial != run.enroll_decentralized.serial
    error_message = "decentralized: serial must change after a successful WebRA renew"
  }
  assert {
    condition     = horizon_certificate.test.thumbprint != run.enroll_decentralized.thumbprint
    error_message = "decentralized: thumbprint must change after a successful WebRA renew"
  }
  # See the centralized note: the test CA stamps not_after at second
  # resolution, so consecutive enrollments may share the exact same value.
  # We only assert no regression; the serial/thumbprint change above
  # already proves a real renewal happened.
  assert {
    condition     = horizon_certificate.test.not_after >= run.enroll_decentralized.not_after
    error_message = "decentralized: renewed cert must not regress not_after"
  }
  # Same logical certificate identity — DN and issuer must match the
  # original enrollment (proves we renewed and didn't re-enroll elsewhere).
  assert {
    condition     = horizon_certificate.test.dn == run.enroll_decentralized.dn
    error_message = "decentralized: dn must be preserved across renewal"
  }
  assert {
    condition     = horizon_certificate.test.issuer == run.enroll_decentralized.issuer
    error_message = "decentralized: issuer must be preserved across renewal"
  }
}
