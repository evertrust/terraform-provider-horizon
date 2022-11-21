# horizon-provider-terraform

The Horizon Provider allows [Terraform](https://terraform.io) to manage Horizon resources.

## Usage Example

```terraform
# 1. Configure the Horizon Provider
provider "horizon" {
  x_api_id  = "myId"
  x_api_key = "myKey"
  endpoint  = "horizon-endpoint"
}

# 2. Create a resource Certificate
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
}
```
