# Horizon Certificate

This resource can enroll a certificate with your rights in Horizon

## Example usage 

```terraform
resource "horizon_certificate" "example" {
  profile   = "Enrollment Profile"
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
  key_type  = "rsa-2048"  
  revoke_on_delete = false
}
```

## Argument Reference

* `profile` - (Required) The enrollment profile.
* `subject` - (Optional) A subject element for your certificate. You can inform multiple subjects to form the complete dn.
    * `element` - (Required) An element of your dn.
    * `type`    - (Required) The type of the element.
    * `value`   - (Required) The value of the element.

* `sans` - (Optional) A san element for your certificate. You can inform multiple sans add to your certificate, it depends on your enrollment profile.
    * `element` - (Required) The element of your san.
    * `type`    - (Required) The type of the element.
    * `value`   - (Required) The value of the element.

* `labels` - (Optional) A label for your certificate. You can inform multiple labels, it depends on your enrollment profile.
    * `label` - (Required) An name of your label.
    * `value` - (Required) The value of the label.

* `team` - (Optional) The team you'd like to link on your certificate.
* `owner` - (Optional) The owner of the certificate you enroll. By default it will be the owner of the credentials used to connect to horizon.
* `key_type` - (Optional) This is the keyType you'd like to use in the enrollment of the crtificate. It is not compatible with the `csr`argument.
* `csr` - (Optional) A CSR file to use the decentralize enroll on Horizon. Be aware that the certificate will be enrolled with the value of your csr. The arguments `subject` and `sans` won't overwrite the CSR.

* `revoke_on_delete` - (Optional) An option that allows you to delete the resource without causing the revocation of the certificate. By default it is set at _true_.