provider "horizon" {
  x_api_id  = "adu"
  x_api_key = "adu"
  endpoint  = "https://horizon-qa.evertrust.io"
}

resource "horizon_certificate" "test1" {

  subject {
    element = "CN"
    type    = "CN"
    value   = "TestTerraform14"
  }
  sans {
    element = "DNSNAME"
    type    = "DNSNAME"
    value   = "TestTerraform14"
  }
  labels {
    label   = "business_units"
    value   = "aaa"
  }
  profile   = "TerraformTest"
  key_type  = "rsa-2048"
  owner     = "a"
  team      = "b"
}

output "test-id" {
  value     = horizon_certificate.test1.id
}

output "test-cert" {
  value     = horizon_certificate.test1.certificate
}

output "test-key" {
  value     = horizon_certificate.test1.key_type
}