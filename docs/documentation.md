## Requirements

The following requirements are needed by this module:

- <a name="requirement_horizon"></a> [horizon](#requirement\_horizon) (~> 0.0.40)

## Providers

The following providers are used by this module:

- <a name="provider_horizon"></a> [horizon](#provider\_horizon) (0.0.158)

## Modules

No modules.

## Resources

The following resources are used by this module:

- horizon_certificate.example (resource)

## Required Inputs

The following input variables are required:

### <a name="input_csr"></a> [csr](#input\_csr)

Description: CSR you'd like to enroll

Type: `string`

### <a name="input_key_type"></a> [key\_type](#input\_key\_type)

Description: Key type, to use only with centralized enrollment

Type: `string`

### <a name="input_labels"></a> [labels](#input\_labels)

Description: Labels of the certificate ; contain an label and a value.

Type: `IndexedDNElement`

### <a name="input_owner"></a> [owner](#input\_owner)

Description: Certificate owner

Type: `string`

### <a name="input_profile"></a> [profile](#input\_profile)

Description: Enrollment profile

Type: `string`

### <a name="input_sans"></a> [sans](#input\_sans)

Description: SAN of the certificate ; contain an element, a type and a value.

Type: `IndexedSANElement`

### <a name="input_subject"></a> [subject](#input\_subject)

Description: Subject of the certificate ; contain an element, a type and a value.

Type: `IndexedDNElement`

### <a name="input_team"></a> [team](#input\_team)

Description: Certificate team

Type: `string`

## Optional Inputs

The following input variables are optional (have default values):

### <a name="input_revoke_on_delete"></a> [revoke\_on\_delete](#input\_revoke\_on\_delete)

Description: option to permit the automatic revocation

Type: `bool`

Default: `"true"`

## Outputs

No outputs.
