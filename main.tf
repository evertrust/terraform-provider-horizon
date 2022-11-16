provider "horizon" {
  x_api_id  = "adu"
  x_api_key = "adu"
  endpoint  = "https://horizon-qa.evertrust.io"
}

resource "horizon_certificate" "test" {

  subject {
    element = "CN"
    type    = "CN"
    value   = "TestTerraform16"
  }
  sans {
    element = "DNSNAME"
    type    = "DNSNAME"
    value   = "TestTerraform16"
  }
  labels {
    label   = "business_units"
    value   = "aaa"
  }
  profile   = "TerraformTest"
  key_type  = "rsa-2048"
  csr       = <<EOT
-----BEGIN CERTIFICATE REQUEST-----
MIICxDCCAawCAQAwUTELMAkGA1UEBhMCRlIxEzARBgNVBAgMClNvbWUtU3RhdGUx
EjAQBgNVBAoMCUV2ZXJUcnVzdDEZMBcGA1UEAwwQVGVzdFRlcnJhZm9ybUNTUjCC
ASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEBAKW7Bc6gQOUzB7ZTSJel3n1v
4y8Fg36tXwESd+D9N+ro1DofGyq6kbUhkUqQBJWlhHtF6kTEWAly6Wp0FGgME3Tr
vBuFGcWmK/cvc+eLmib6c4BVoSOB5sS055wBh/vZ2BEcr8/mfse4y4mDbzsAzbGp
LalNoOVbCJCKbG+Q0L9gILXrYpinLlSiXzZTn603E+/K30hIvuE2HDc2JvjRNefN
AUfZ9zcj3ASWQx8DB3eASqgrHnU6RIZTmL7eWfpzjzrZr1kUMzb4yzDqKj3y4one
YH/QAPMLx+TyjHLwfcxoJTL0UZ+WI0dINXH8qZVQUOM4lh9Flli1+1R7dmX8W/cC
AwEAAaAuMCwGCSqGSIb3DQEJDjEfMB0wGwYDVR0RBBQwEoIQdGVzdFRlcnJhZm9y
bUNTUjANBgkqhkiG9w0BAQsFAAOCAQEAj5RNtqHDrVClnoUv/OIzmAmyg9UvNTn6
iXNcO5Z+tELlREYBwz4TYFhYsNBUNXfwwLSkWok0wwTBqGuvcCbVuqPGc8Xybz3q
seVeaz5wQWlPdHfT0a5z0u9PEpJsFHA7gI0zM56hKPQJb2uU/ZoTy5MBzwubaUOH
TreylFrb07SvN/gpcdU31Uulr5enQvHjYJhBTlCx6yLglkoARVfrgWQVhdx/T1m1
JG7RU8PvDLymlyYEqYXzhvJBSUAQBlCPBHhrEUsUrS97IDcBDXR+tS03tDO3fvKk
l/UlydOTQcxGMMcliu4GeQzh6RhqB3IcQttqwXdIQh1HRiUZPIv6rQ==
-----END CERTIFICATE REQUEST-----
EOT
}

resource "horizon_certificate" "test2" {
  subject {
    element = "CN"
    type    = "CN"
    value   = "TestTerraformFill"
  }
  sans {
    element = "DNSNAME"
    type    = "DNSNAME"
    value   = "TestTerraformFill"
  }
  labels {
    label   = "business_units"
    value   = "aaa"
  }
  profile   = "TerraformTest"
  key_type  = "rsa-2048"  
}

output "test-id" {
  value     = horizon_certificate.test2.id
}

output "test-cert" {
  value     = horizon_certificate.test2.certificate
}

output "test-key" {
  value     = horizon_certificate.test2.key_type
}

output "not-after" {
  value     = horizon_certificate.test2.not_after
}