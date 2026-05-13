# Resolve the trust chain of a certificate already managed by the provider.
resource "horizon_certificate" "example" {
  profile = "EnrollmentProfile"

  subject = [
    {
      element = "CN"
      type    = "CN"
      value   = "example.com"
    }
  ]
}

data "horizon_certificate_trust_chain" "from_resource" {
  certificate_pem = horizon_certificate.example.certificate
  order           = "leaf_to_root"
}

output "resource_chain_pem" {
  value = data.horizon_certificate_trust_chain.from_resource.chain_pem
}

# Resolve the trust chain of a certificate read from a file.
data "horizon_certificate_trust_chain" "from_file" {
  certificate_pem = file("${path.module}/certs/leaf.pem")
  order           = "root_to_leaf"
}

output "file_chain" {
  value = data.horizon_certificate_trust_chain.from_file.chain
}
