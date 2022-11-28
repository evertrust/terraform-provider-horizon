---
page_title: "Provider: Horizon"
description: |-
   The Horizon provider is used to manage your certificates
---

# Horizon Provider 

The Horizon Provider is used to manage your certificates.

## Example usage

```terraform
provider "horizon" {
  x_api_id  = "example"
  x_api_key = "example"
  endpoint  = "https://horizon.example"
}
```

## Argument Reference

* `x_api_id`- (Required) Your horizon id.
* `x_api_key`- (Required) Your horizon password.
* `endpoint`- (Required) The horizon url.