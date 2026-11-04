terraform {
  required_providers {
    horizon = {
      source = "registry.terraform.io/evertrust/horizon"
    }
  }
}

# With creds authentication
provider "horizon" {
  alias = "with-creds"

  endpoint = "https://horizon.company.com"
  username = "username"
  password = "password"
}

# With certificate authentication
provider "horizon" {
  alias           = "with-cert"
  endpoint        = "https://horizon.company.com"
  client_cert_pem = "-----BEGIN CERTIFICATE-----\n...\n-----END CERTIFICATE-----\n"
  client_key_pem  = "-----BEGIN PRIVATE KEY-----\n...\n-----END PRIVATE KEY-----\n"
}
