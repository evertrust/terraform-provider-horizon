package horizon

import (
	"context"
	"crypto/tls"
	"fmt"

	"github.com/evertrust/horizon-go"
	old "github.com/evertrust/horizon-go/v2"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"gopkg.in/resty.v1"
)

func Provider() *schema.Provider {
	return &schema.Provider{
		Schema: map[string]*schema.Schema{
			"x_api_id": {
				Description: "Local account identifier.",
				Type:        schema.TypeString,
				Optional:    true,
			},
			"x_api_key": {
				Description: "Local account password.",
				Type:        schema.TypeString,
				Optional:    true,
			},
			"cert": {
				Description: "Authent certificate file.",
				Type:        schema.TypeString,
				Optional:    true,
			},
			"key": {
				Description: "Key file for the authent by certificate.",
				Type:        schema.TypeString,
				Optional:    true,
			},
			"endpoint": {
				Description: "Horizon endpoint.",
				Type:        schema.TypeString,
				Required:    true,
			},
			"horizon_version": {
				Description: "Horizon version. Useful for version before 2.4.",
				Type:        schema.TypeFloat,
				Optional:    true,
				Computed:    false,
				Default:     2.4,
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

	x_api_id := d.Get("x_api_id").(string)
	x_api_key := d.Get("x_api_key").(string)
	cert := d.Get("cert").(string)
	key := d.Get("key").(string)
	endpoint, _ := d.Get("endpoint").(string)

	r := resty.New()

	if cert != "" && key != "" {
		clientCert, err := tls.LoadX509KeyPair(cert, key)
		if err != nil {
			diags = append(diags, diag.Diagnostic{
				Severity: diag.Error,
				Summary:  fmt.Sprintf("ERROR: %s", err),
			})
			return nil, diags
		}
		r.SetCertificates(clientCert)
	}

	r.SetHeader("Content-Type", "application/json").
		SetHostURL(endpoint).
		SetHeader("X-API-ID", x_api_id).
		SetHeader("X-API-KEY", x_api_key).
		SetCookieJar(nil)

	if d.Get("horizon_version").(float64) >= 2.4 {
		c := new(horizon.Horizon)
		c.Init(r)
		return c, diags
	} else {
		o := new(old.Horizon)
		o.Init(r)
		return o, diags
	}
}
