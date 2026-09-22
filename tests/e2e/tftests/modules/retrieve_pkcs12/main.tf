terraform {
  required_providers {
    horizon = {
      source = "registry.terraform.io/evertrust/horizon"
    }
  }
}

provider "horizon" {
  endpoint = var.endpoint
  username = var.username
  password = var.password
}

resource "horizon_certificate" "test" {
  profile             = var.profile
  key_type            = "rsa-2048"
  renew_before        = var.renew_before_days
  pkcs12_write_only   = true
  password_write_only = true

  subject = [
    {
      element = "cn.1"
      type    = "CN"
      value   = var.cn
    }
  ]
}

ephemeral "horizon_retrieve_centralized_pkcs12" "test" {
  certificate_id    = horizon_certificate.test.id
  skip_escrow_check = var.skip_escrow_check
}

# terraform test cannot assert on ephemeral resources from a run block, so the
# expectations live here: a failing check block fails the run.
check "pkcs12_retrieved" {
  assert {
    condition = (
      ephemeral.horizon_retrieve_centralized_pkcs12.test.pkcs12 != "" &&
      ephemeral.horizon_retrieve_centralized_pkcs12.test.password != ""
    )
    error_message = "pkcs12 and password must not be empty"
  }

  assert {
    condition     = ephemeral.horizon_retrieve_centralized_pkcs12.test.certificate_id == horizon_certificate.test.id
    error_message = "the ephemeral resource must target the current certificate"
  }

  assert {
    condition     = ephemeral.horizon_retrieve_centralized_pkcs12.test.source == var.expected_source
    error_message = "PKCS#12 material was not served by the expected Horizon request (expected source: ${var.expected_source})"
  }
}

output "id" {
  value = horizon_certificate.test.id
}

output "serial" {
  value = horizon_certificate.test.serial
}
