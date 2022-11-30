provider "horizon" {
  x_api_id  = "example"
  x_api_key = "example"
  endpoint  = "https://horizon.example"
}

resource "horizon_certificate" "example" {
  subject {
    element = "CN"
    type    = "CN"
    value   = "example.terraform.cn"
  }
  sans {
    element = "DNSNAME"
    type    = "DNSNAME"
    value   = "example.terraform.dnsname"
  }
  labels {
    label   = "label"
    value   = "example"
  }
  profile   = "Enrollment Profile"
  key_type  = "rsa-2048"  
  revoke_on_delete = false
}