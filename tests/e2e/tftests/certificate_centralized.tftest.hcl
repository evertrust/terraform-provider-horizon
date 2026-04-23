# Centralized enrollment scenarios. Each run block targets the child module
# ./modules/centralized with a different set of write-only flags, then asserts
# on the resource state after apply.

variable "endpoint" { type = string }
variable "username" { type = string }
variable "password" {
  type      = string
  sensitive = true
}
variable "centralized_profile" { type = string }

run "write_only_both" {
  command = apply

  module {
    source = "./modules/centralized"
  }

  variables {
    endpoint            = var.endpoint
    username            = var.username
    password            = var.password
    profile             = var.centralized_profile
    cn                  = "write-only-both.tf-test.internal"
    pkcs12_write_only   = true
    password_write_only = true
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
    condition     = horizon_certificate.test.pkcs12 == null
    error_message = "pkcs12 must be null when pkcs12_write_only=true"
  }
  assert {
    condition     = horizon_certificate.test.password == null
    error_message = "password must be null when password_write_only=true"
  }
}

run "write_only_pkcs12" {
  command = apply

  module {
    source = "./modules/centralized"
  }

  variables {
    endpoint            = var.endpoint
    username            = var.username
    password            = var.password
    profile             = var.centralized_profile
    cn                  = "write-only-pkcs12.tf-test.internal"
    pkcs12_write_only   = true
    password_write_only = false
  }

  assert {
    condition     = horizon_certificate.test.id != ""
    error_message = "certificate id must be set"
  }
  assert {
    condition     = horizon_certificate.test.pkcs12 == null
    error_message = "pkcs12 must be null when pkcs12_write_only=true"
  }
  assert {
    condition     = horizon_certificate.test.password != null && horizon_certificate.test.password != ""
    error_message = "password must be retained when password_write_only=false"
  }
}

run "write_only_password" {
  command = apply

  module {
    source = "./modules/centralized"
  }

  variables {
    endpoint            = var.endpoint
    username            = var.username
    password            = var.password
    profile             = var.centralized_profile
    cn                  = "write-only-password.tf-test.internal"
    pkcs12_write_only   = false
    password_write_only = true
  }

  assert {
    condition     = horizon_certificate.test.id != ""
    error_message = "certificate id must be set"
  }
  assert {
    condition     = horizon_certificate.test.pkcs12 != null && horizon_certificate.test.pkcs12 != ""
    error_message = "pkcs12 must be retained when pkcs12_write_only=false"
  }
  assert {
    condition     = horizon_certificate.test.password == null
    error_message = "password must be null when password_write_only=true"
  }
}

run "default_behavior" {
  command = apply

  module {
    source = "./modules/centralized"
  }

  variables {
    endpoint            = var.endpoint
    username            = var.username
    password            = var.password
    profile             = var.centralized_profile
    cn                  = "default-behavior.tf-test.internal"
    pkcs12_write_only   = false
    password_write_only = false
  }

  assert {
    condition     = horizon_certificate.test.id != ""
    error_message = "certificate id must be set"
  }
  assert {
    condition     = horizon_certificate.test.pkcs12 != null && horizon_certificate.test.pkcs12 != ""
    error_message = "pkcs12 must be retained with default flags"
  }
  assert {
    condition     = horizon_certificate.test.password != null && horizon_certificate.test.password != ""
    error_message = "password must be retained with default flags"
  }
}
