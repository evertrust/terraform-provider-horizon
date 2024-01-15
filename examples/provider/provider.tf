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
  alias    = "with-cert"
  endpoint = "https://horizon.company.com"
  cert     = "----BEGIN CERTIFICATE-----\n...\n----END CERTIFICATE-----\n"
  key      = "----BEGIN RSA PRIVATE KEY-----\n...\n----END RSA PRIVATE KEY-----"
}
