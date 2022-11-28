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

variable "subject" {
  description = "Subject of the certificate ; contain an element, a type and a value."
  type = IndexedDNElement
}

variable "sans" {
  description = "SAN of the certificate ; contain an element, a type and a value."
  type = IndexedSANElement
}

variable "labels" {
  description = "Labels of the certificate ; contain an label and a value."
  type = IndexedDNElement
}

variable "profile" {
  description = "Enrollment profile"
  type = string
}

variable "key_type" {
  description = "Key type, to use only with centralized enrollment"
  type = string
}

variable "owner" {
  description = "Certificate owner"
  type = string
}

variable "team" {
  description = "Certificate team"
  type = string
}

variable "csr" {
  description = "CSR you'd like to enroll"
  type = csr
}

variable "revoke_on_delete" {
  description = "option to permit the automatic revocation"
  default = true
  type = bool
}