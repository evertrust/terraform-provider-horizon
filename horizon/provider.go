package horizon

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net/url"

	horizon "github.com/evertrust/horizon-go/client"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func Provider() *schema.Provider {
	return &schema.Provider{
		Schema: map[string]*schema.Schema{
			"endpoint": {
				Description: "Horizon URL, with protocol (https://) and without trailing slash. Required.",
				Type:        schema.TypeString,
				Required:    true,
			},
			"username": {
				Description: "Local account identifier. Required when password is provided.",
				Type:        schema.TypeString,
				Optional:    true,
			},
			"password": {
				Description: "Local account password. Required when username is provided.",
				Type:        schema.TypeString,
				Optional:    true,
				Sensitive:   true,
			},
			"client_cert_pem": {
				Description: "Client certificate to use for authentication. Required when client_key_pem is provided.",
				Type:        schema.TypeString,
				Optional:    true,
			},
			"client_key_pem": {
				Description: "Private key associated with the client certificate. Required when client_cert_pem is provided.",
				Type:        schema.TypeString,
				Optional:    true,
				Sensitive:   true,
			},
			"ca_bundle_pem": {
				Description: "PEM-encoded CA bundle to use for TLS verification. Optional.",
				Type:        schema.TypeString,
				Optional:    true,
			},
			"skip_tls_verify": {
				Description: "Skip TLS verification. Not recommended in production. Optional.",
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
			},
		},
		ResourcesMap: map[string]*schema.Resource{
			"horizon_certificate": resourceCertificate(),
		},
		DataSourcesMap:       map[string]*schema.Resource{},
		ConfigureContextFunc: providerConfigure,
	}
}

func providerConfigure(ctx context.Context, d *schema.ResourceData) (interface{}, diag.Diagnostics) {
	// Warning or errors can be collected in a slice type
	var diags diag.Diagnostics

	client := horizon.New(nil)

	endpoint, err := url.Parse(d.Get("endpoint").(string))
	if err != nil {
		return nil, diag.FromErr(err)
	}
	client.SetBaseUrl(*endpoint)

	caBundle, hasCaBundle := d.GetOk("ca_bundle_pem")
	if hasCaBundle {
		pool := x509.NewCertPool()
		pool.AppendCertsFromPEM([]byte(caBundle.(string)))
		client.SetCaBundle(pool)
	}

	skipTlsVerify := d.Get("skip_tls_verify").(bool)
	if skipTlsVerify {
		client.SkipTLSVerify()
	}

	username, hasUsername := d.GetOk("username")
	password, hasPassword := d.GetOk("password")
	cert, hasCert := d.GetOk("client_cert_pem")
	key, hasKey := d.GetOk("client_cert_pem")

	if hasUsername {
		if !hasPassword {
			diags = append(diags, diag.Diagnostic{
				Severity: diag.Error,
				Summary:  "password is required when username is provided.",
			})
			return nil, diags
		}
		client.SetPasswordAuth(username.(string), password.(string))
	} else if hasCert {
		if !hasKey {
			diags = append(diags, diag.Diagnostic{
				Severity: diag.Error,
				Summary:  "client_key_pem is required when client_cert_pem is provided.",
			})
			return nil, diags
		}

		parsedCert, err := tls.X509KeyPair([]byte(cert.(string)), []byte(key.(string)))
		if err != nil {
			diags = append(diags, diag.Diagnostic{
				Severity: diag.Error,
				Summary:  "Failed to parse X509 certificate : " + err.Error(),
			})
			return nil, diags
		}
		client.SetCertAuth(parsedCert)
	} else {
		diags = append(diags, diag.Diagnostic{
			Severity: diag.Error,
			Summary:  "No credentials provided.",
		})
		return nil, diags
	}

	return client, diags
}
