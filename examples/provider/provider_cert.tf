provider "horizon" {
  alias = "with-cert"

  endpoint = "https://horizon.company.com"
  cert     = "----BEGIN CERTIFICATE-----\n...\n----END CERTIFICATE-----\n"
  key      = "----BEGIN RSA PRIVATE KEY-----\n...\n----END RSA PRIVATE KEY-----"
}