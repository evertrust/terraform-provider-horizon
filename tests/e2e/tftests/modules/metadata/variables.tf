variable "endpoint" { type = string }
variable "username" { type = string }
variable "password" {
  type      = string
  sensitive = true
}
variable "profile" { type = string }
variable "cn" { type = string }

variable "sans_dns" {
  type    = list(string)
  default = []
}

variable "labels" {
  type    = map(string)
  default = {}
}

variable "owner" {
  type    = string
  default = null
}

variable "team" {
  type    = string
  default = null
}

variable "contact_email" {
  type    = string
  default = null
}

variable "revoke_on_delete" {
  type    = bool
  default = false
}
