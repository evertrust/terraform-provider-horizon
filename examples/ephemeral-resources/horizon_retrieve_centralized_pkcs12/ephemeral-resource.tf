# Enroll a centralized certificate, keeping the PKCS#12 material out of state.
resource "horizon_certificate" "server" {
  profile             = "EnrollmentProfile"
  pkcs12_write_only   = true
  password_write_only = true

  subject = [
    {
      element = "cn.1"
      type    = "CN"
      value   = "server.example.com"
    }
  ]
}

# Retrieve the centralized PKCS#12 bundle on demand. The returned values are
# ephemeral and are never written to Terraform state or saved plan files.
ephemeral "horizon_retrieve_centralized_pkcs12" "server" {
  certificate_id = horizon_certificate.server.id
}

# Hand the bundle off to a downstream write-only consumer.
resource "some_secret_consumer" "server" {
  write_only_pkcs12   = ephemeral.horizon_retrieve_centralized_pkcs12.server.pkcs12
  write_only_password = ephemeral.horizon_retrieve_centralized_pkcs12.server.password
}
