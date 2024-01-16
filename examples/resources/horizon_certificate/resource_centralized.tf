resource "horizon_certificate" "example" {
  subject {
    element = "cn.1"
    type    = "CN"
    value   = "example.org"
  }
  sans {
    type    = "DNSNAME"
    value   = ["example.org"]
  }
  labels {
    label   = "label"
    value   = "example"
  }
  profile   = "DefaultProfile"
  key_type  = "rsa-2048"  
  revoke_on_delete = false
}