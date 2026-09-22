variable "endpoint" {
  type = string
}

variable "username" {
  type = string
}

variable "password" {
  type      = string
  sensitive = true
}

variable "profile" {
  type = string
}

variable "cn" {
  type = string
}

variable "challenge" {
  type        = string
  sensitive   = true
  default     = null
  description = "One-time WebRA challenge issued beforehand on var.profile."
}

variable "request_challenge" {
  type    = bool
  default = null
}

variable "renew_before_days" {
  type    = number
  default = null
}

variable "contact_email" {
  type    = string
  default = null
}
