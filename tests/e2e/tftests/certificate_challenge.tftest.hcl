variable "endpoint" { type = string }
variable "username" { type = string }
variable "password" {
  type      = string
  sensitive = true
}
variable "challenge_centralized_profile" { type = string }
variable "challenge_decentralized_profile" { type = string }
variable "challenge_template_profile" { type = string }
variable "issued_challenge" {
  type      = string
  sensitive = true
}

run "provided_challenge" {
  command = apply

  module {
    source = "./modules/challenge"
  }

  variables {
    endpoint  = var.endpoint
    username  = var.username
    password  = var.password
    profile   = var.challenge_centralized_profile
    cn        = "challenge-provided.tf-test.internal"
    challenge = var.issued_challenge
  }

  assert {
    condition     = horizon_certificate.test.id != ""
    error_message = "the provider must resolve the Horizon id of a certificate enrolled with a challenge"
  }
  assert {
    condition     = horizon_certificate.test.dn == "CN=challenge-provided.tf-test.internal"
    error_message = "on an empty-template profile, the certificate must carry the identity sent with the challenge"
  }
  assert {
    condition     = horizon_certificate.test.pkcs12 != null && horizon_certificate.test.pkcs12 != ""
    error_message = "centralized challenge enrollment must return the PKCS#12"
  }
  assert {
    condition     = horizon_certificate.test.password == var.issued_challenge
    error_message = "the PKCS#12 of a challenge enrollment is encrypted with the challenge"
  }
}


run "provided_challenge_no_drift" {
  command = apply

  module {
    source = "./modules/challenge"
  }

  variables {
    endpoint  = var.endpoint
    username  = var.username
    password  = var.password
    profile   = var.challenge_centralized_profile
    cn        = "challenge-provided.tf-test.internal"
    challenge = var.issued_challenge
  }

  assert {
    condition     = horizon_certificate.test.id == run.provided_challenge.id
    error_message = "id changed on a second apply with a consumed challenge"
  }
  assert {
    condition     = horizon_certificate.test.serial == run.provided_challenge.serial
    error_message = "serial changed on a second apply with a consumed challenge"
  }
}


run "provided_challenge_renew" {
  command = apply

  module {
    source = "./modules/challenge"
  }

  variables {
    endpoint          = var.endpoint
    username          = var.username
    password          = var.password
    profile           = var.challenge_centralized_profile
    cn                = "challenge-provided.tf-test.internal"
    challenge         = var.issued_challenge
    renew_before_days = 400
  }

  assert {
    condition     = horizon_certificate.test.serial != run.provided_challenge.serial
    error_message = "a certificate enrolled with a challenge must still renew through WebRA"
  }
  assert {
    condition     = horizon_certificate.test.dn == "CN=challenge-provided.tf-test.internal"
    error_message = "dn must be preserved across renewal"
  }
}

run "request_challenge_empty_template" {
  command = apply

  module {
    source = "./modules/challenge"
  }

  variables {
    endpoint          = var.endpoint
    username          = var.username
    password          = var.password
    profile           = var.challenge_centralized_profile
    cn                = "challenge-requested.tf-test.internal"
    request_challenge = true
    contact_email     = "challenge@tf-test.internal"
  }

  assert {
    condition     = horizon_certificate.test.dn == "CN=challenge-requested.tf-test.internal"
    error_message = "on an empty-template profile, the provider must send the identity when it consumes the challenge"
  }
  assert {
    condition     = horizon_certificate.test.pkcs12 != null && horizon_certificate.test.password != null
    error_message = "centralized challenge enrollment must return the PKCS#12 and its password"
  }
}

run "request_challenge_defined_template" {
  command = apply

  module {
    source = "./modules/challenge"
  }

  variables {
    endpoint          = var.endpoint
    username          = var.username
    password          = var.password
    profile           = var.challenge_template_profile
    cn                = "challenge-template.tf-test.internal"
    request_challenge = true
  }

  assert {
    condition     = horizon_certificate.test.dn == "CN=challenge-template.tf-test.internal"
    error_message = "on a defined-template profile, the provider must send the identity when it issues the challenge"
  }
  assert {
    condition     = horizon_certificate.test.pkcs12 != null
    error_message = "centralized challenge enrollment must return the PKCS#12"
  }
}

run "request_challenge_decentralized" {
  command = apply

  module {
    source = "./modules/challenge-decentralized"
  }

  variables {
    endpoint = var.endpoint
    username = var.username
    password = var.password
    profile  = var.challenge_decentralized_profile
    cn       = "challenge-decentralized.tf-test.internal"
  }

  assert {
    condition     = horizon_certificate.test.certificate != ""
    error_message = "decentralized challenge enrollment must return the certificate"
  }
  # The provider reads the DN and SAN from the CSR and sends them with it.
  assert {
    condition     = strcontains(horizon_certificate.test.dn, "CN=challenge-decentralized.tf-test.internal") && strcontains(horizon_certificate.test.dn, "O=tf-test")
    error_message = "the certificate must carry the identity found in the CSR"
  }
  assert {
    condition     = horizon_certificate.test.pkcs12 == null && horizon_certificate.test.password == null
    error_message = "decentralized enrollment must not return a PKCS#12"
  }
}
