---
page_title: "horizon_certificate Resource - horizon"
subcategory: ""
description: |-
  Provides a Certificate resource. This resource allow you to manage the life cycle of a certificate.
---

# Horizon Certificate Resource

The `horizon_certificate` resource in Terraform allows you to create, manage, and enroll digital certificates using Horizon's centralized or decentralized methods. This resource is designed to provide a robust and flexible way to handle certificate lifecycle management.

## Table of Contents

- [Authentication Methods](#authentication-methods)
  - [Centralized Enrollment](#centralized-enrollment)
  - [Decentralized Enrollment](#decentralized-enrollment)
- [Resource Schema](#resource-schema)
  - [Required Parameters](#required-parameters)
  - [Optional Parameters](#optional-parameters)
  - [Read-Only Parameters](#read-only-parameters)

## Authentication Methods

### Centralized Enrollment

In centralized enrollment, the certificate is created and managed by Horizon. The `key_file` argument is required for this method.

```terraform
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
    label = "label"
    value = "example"
  }
  profile          = "Enrollment Profile"
  key_type         = "rsa-2048"
  revoke_on_delete = false
}
```

### Parameters

- `subject` (Block Set) Subject element for the certificate.
- `sans` (Block Set) SAN element for the certificate.
- `labels` (Block Set) Labels for the certificate. 
- `profile` (String) Enrollment profile.
- `key_type` (String) This is the keyType you'd like to use in the enrollment of the crtificate. It is not compatible
  with the `csr`argument.
- `revoke_on_delete` (Boolean) An option that allows you to delete the resource without causing the revocation of the
  certificate.


### Decentralized Enrollment

In decentralized enrollment, you provide your own Certificate Signing Request (CSR) to Horizon. This method offers more control over the certificate attributes and is ideal for environments with specific security requirements. Note that the `key_file` argument is not applicable in this method.

```terraform
resource "horizon_certificate" "example" {
  labels {
    label = "label"
    value = "example"
  }
  profile          = "Enrollment Profile"
  csr              = <<EOT
CSR CONTENT
  EOT
  revoke_on_delete = false
}

```

### Parameters

- `labels` (Block Set) Labels for the certificate.
- `profile` (String) Enrollment profile.
- `csr` (String) A CSR file to use the decentralized enroll on Horizon. Be aware that the certificate will be enrolled
  with the value of your csr. The arguments `subject` and `sans` won't overwrite the CSR.
- `revoke_on_delete` (Boolean) An option that allows you to delete the resource without causing the revocation of the
  certificate.
