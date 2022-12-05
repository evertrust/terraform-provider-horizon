resource "horizon_certificate" "example" {
  labels {
    label   = "label"
    value   = "example"
  }
  profile   = "Enrollment Profile"
  csr = <<EOT
CSR CONTENT
  EOT
  revoke_on_delete = false
}