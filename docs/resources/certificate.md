---
# generated by https://github.com/hashicorp/terraform-plugin-docs
page_title: "horizon_certificate Resource - terraform-provider-horizon"
subcategory: ""
description: |-
  Provides a Certificate resource. This resource allow you to manage the lifecycle of a certificate. To enroll a certificate, you can either provide a CSR (Certificate Signing Request), or a subject and a list of SANs. If you provide a CSR, the enrollment will be decentralized. If you provide a subject and SANs, the enrollment will be centralized.
---

# horizon_certificate (Resource)

Provides a Certificate resource. This resource allow you to manage the lifecycle of a certificate. To enroll a certificate, you can either provide a CSR (Certificate Signing Request), or a subject and a list of SANs. If you provide a CSR, the enrollment will be decentralized. If you provide a subject and SANs, the enrollment will be centralized.

## Example Usage

```terraform
# Centralized enrollement
resource "horizon_certificate" "example_centralized" {
  profile          = "EnrollmentProfile"
  key_type         = "rsa-2048"
  revoke_on_delete = true
  renew_before     = 30

  subject = [
    {
      element = "CN"
      type    = "CN"
      value   = "example.com"
    }
  ]
  sans = [
    {
      type  = "DNSNAME"
      value = ["example.com", "www.example.com"]
    }
  ]
  labels = [
    {
      label = "labelKey"
      value = "labelValue"
    }
  ]
}

# Decentralized enrollment
resource "tls_private_key" "example_decentralized" {
  algorithm = "RSA"
  rsa_bits  = 2048
}

resource "tls_cert_request" "example_decentralized" {
  private_key_pem = tls_private_key.example_decentralized.private_key_pem

  subject {
    common_name  = "example_decentralized.com"
    organization = "ACME Examples, Inc"
  }
}

resource "horizon_certificate" "example_decentralized" {
  csr              = tls_cert_request.example_decentralized.cert_request_pem
  profile          = "EnrollmentProfile"
  revoke_on_delete = true

  labels = [
    {
      label = "labelKey"
      value = "labelValue"
    }
  ]
}
```

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `profile` (String) Profile where the certificate will be enrolled into.

### Optional

- `certificate` (String) Certificate in the PEM format.
- `contact_email` (String) Contact email associated with the certificate.
- `csr` (String) A CSR (Certificate Signing Request) in PEM format. Providing this attribute will trigger a decentralized enrollment. Incompatible with `subject` and `sans`.
- `key_type` (String) Key type of the certificate. For example: `rsa-2048`.
- `labels` (Attributes Set) Labels of the certificate, used to enrich the certificate metadata on Horizon. (see [below for nested schema](#nestedatt--labels))
- `owner` (String) Owner associated with the certificate.
- `password` (String, Sensitive) Password of the PKCS12 file. Can be provided when using centralized enrollment, or will be generated by Horizon if not set.
- `pkcs12` (String, Sensitive) Base64-encoded PKCS12 file containing the certificate and the private key. Provided when using centralized enrollment.
- `renew_before` (Number) How many days to renew the certificate before it expires. Certificate renewals rely on the Terraform workspace being run regularly. If the workspace is not run, the certificate will expire.
- `revoke_on_delete` (Boolean) Whether to revoke certificate when it is removed from the Terraform state or not.
- `sans` (Attributes Set) Subject alternative names of the certificate. This is ignored when csr is provided. (see [below for nested schema](#nestedatt--sans))
- `subject` (Attributes Set) Subject elements of the certificate. This is ignored when csr is provided. (see [below for nested schema](#nestedatt--subject))
- `team` (String) Team associated with the certificate.

### Read-Only

- `dn` (String) DN of the certificate.
- `id` (String) Internal certificate identifier.
- `issuer` (String) Issuer DN of the certificate.
- `not_after` (Number) NotAfter attribute (expiration date) of the certificate.
- `not_before` (Number) NotBefore attribute of the certificate.
- `public_key_thumbprint` (String) Public key thumbprint of the certificate.
- `revocation_date` (Number) Revocation date of the certificate. Empty when the certificate is not revoked.
- `self_signed` (Boolean) Whether this is a self-signed certificate.
- `serial` (String) Serial number of the certificate.
- `signing_algorithm` (String) Signing algorithm of the certificate. For example: `SHA256WITHRSA`
- `thumbprint` (String) Thumbprint of the certificate.

<a id="nestedatt--labels"></a>
### Nested Schema for `labels`

Required:

- `label` (String) Label name.
- `value` (String) Label value.


<a id="nestedatt--sans"></a>
### Nested Schema for `sans`

Required:

- `type` (String) SAN type. Accepted values are: `RFC822NAME`, `DNSNAME`, `URI`, `IPADDRESS`, `OTHERNAME_UPN`, `OTHERNAME_GUID`
- `value` (Set of String) Subject element values.


<a id="nestedatt--subject"></a>
### Nested Schema for `subject`

Required:

- `element` (String) Subject element, followed by a dot and the index of the element. For example: `cn.1` for the first common name.
- `type` (String) Subject element type. For example: `CN` for common name.
- `value` (String) Subject element value. For example: `www.example.com` for common name.
