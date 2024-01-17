package provider

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"github.com/evertrust/horizon-go"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"net/url"
)

// Ensure HorizonProvider satisfies various provider interfaces.
var _ provider.Provider = &HorizonProvider{}

// HorizonProvider defines the provider implementation.
type HorizonProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" when running acceptance
	// testing.
	version string
}

// horizonProviderModel describes the provider data model.
type horizonProviderModel struct {
	Endpoint      types.String `tfsdk:"endpoint"`
	Username      types.String `tfsdk:"username"`
	Password      types.String `tfsdk:"password"`
	ClientCertPem types.String `tfsdk:"client_cert_pem"`
	ClientKeyPem  types.String `tfsdk:"client_key_pem"`
	SkipTlsVerify types.Bool   `tfsdk:"skip_tls_verify"`
	CaBundlePem   types.String `tfsdk:"ca_bundle_pem"`
}

func (p *HorizonProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "horizon"
	resp.Version = p.version
}

func (p *HorizonProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "The Horizon provider is used to interact with the Horizon API. It can manage the lifecycle of certificates and other resources.",
		Attributes: map[string]schema.Attribute{
			"endpoint": schema.StringAttribute{
				MarkdownDescription: "Horizon URL, with protocol (https://) and without trailing slash. Required.",
				Required:            true,
			},
			"username": schema.StringAttribute{
				MarkdownDescription: "Local account identifier. Required when password is provided.",
				Optional:            true,
			},
			"password": schema.StringAttribute{
				MarkdownDescription: "Local account password. Required when username is provided.",
				Optional:            true,
			},
			"client_cert_pem": schema.StringAttribute{
				MarkdownDescription: "Client certificate to use for authentication. Required when client_key_pem is provided.",
				Optional:            true,
			},
			"client_key_pem": schema.StringAttribute{
				MarkdownDescription: "Private key associated with the client certificate. Required when client_cert_pem is provided.",
				Optional:            true,
			},
			"skip_tls_verify": schema.BoolAttribute{
				MarkdownDescription: "Skip TLS certificate verification. Optional, default to false. Note that this is not recommended in production.",
				Optional:            true,
			},
			"ca_bundle_pem": schema.StringAttribute{
				MarkdownDescription: "PEM-encoded CA bundle to use for TLS certificate verification. Optional.",
				Optional:            true,
			},
		},
	}
}

func (p *HorizonProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data horizonProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	client := horizon.New(horizon.NewHttpClient())

	endpoint, err := url.Parse(data.Endpoint.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid endpoint URL", err.Error())
		return
	}
	client.Http.SetBaseUrl(*endpoint)

	if data.SkipTlsVerify.ValueBool() {
		client.Http.SkipTLSVerify()
	}

	if !data.CaBundlePem.IsNull() {
		pool := x509.NewCertPool()
		pool.AppendCertsFromPEM([]byte(data.CaBundlePem.ValueString()))
		client.Http.SetCaBundle(pool)
	}

	if !data.Username.IsNull() {
		if data.Password.IsNull() {
			resp.Diagnostics.AddError("Password is required when username is provided.", "")
			return
		}
		client.Http.SetPasswordAuth(data.Username.ValueString(), data.Password.ValueString())
	} else if !data.ClientCertPem.IsNull() {
		if data.ClientKeyPem.IsNull() {
			resp.Diagnostics.AddError("client_key_pem is required when client_cert_pem is provided.", "")
			return
		}

		parsedCert, err := tls.X509KeyPair([]byte(data.ClientCertPem.ValueString()), []byte(data.ClientKeyPem.ValueString()))
		if err != nil {
			resp.Diagnostics.AddError("Failed to load TLS certificate", err.Error())
			return
		}
		client.Http.SetCertAuth(parsedCert)
	} else {
		resp.Diagnostics.AddError("No authentication method provided", "Please provide either username/password or client_cert_pem/client_key_pem.")
		return
	}

	resp.DataSourceData = client
	resp.ResourceData = client
}

func (p *HorizonProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewCertificateResource,
	}
}

func (p *HorizonProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		//NewExampleDataSource,
	}
}

func (p HorizonProvider) ValidateConfig(ctx context.Context, req provider.ValidateConfigRequest, resp *provider.ValidateConfigResponse) {
	var data horizonProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if !data.Username.IsNull() {
		// We assume we're in a creds auth mode, so password is required.
		if data.Password.IsNull() {
			resp.Diagnostics.AddAttributeError(path.Root("username"), "Password is required when username is provided.", "")
			return
		}

		if !data.ClientCertPem.IsNull() {
			resp.Diagnostics.AddAttributeError(path.Root("client_cert_pem"), "Client certificate is not supported when username is provided.", "")
			return
		}

		if !data.ClientKeyPem.IsNull() {
			resp.Diagnostics.AddAttributeError(path.Root("client_key_pem"), "Client key is not supported when username is provided.", "")
			return
		}
	} else if !data.ClientCertPem.IsNull() {
		// We assume we're in a cert auth mode, so client_key_pem is required.
		if data.ClientKeyPem.IsNull() {
			resp.Diagnostics.AddAttributeError(path.Root("client_cert_pem"), "client_key_pem is required when client_cert_pem is provided.", "")
			return
		}

		if !data.Password.IsNull() {
			resp.Diagnostics.AddAttributeError(path.Root("password"), "Password is not supported when client_cert_pem is provided.", "")
			return
		}
	} else {
		resp.Diagnostics.AddError("No authentication method provided", "Please provide either username/password or client_cert_pem/client_key_pem.")
		return
	}

	if !data.SkipTlsVerify.IsNull() && data.SkipTlsVerify.ValueBool() && !data.CaBundlePem.IsNull() {
		resp.Diagnostics.AddAttributeWarning(path.Root("skip_tls_verify"), "skip_tls_verify is not recommended when ca_bundle_pem is provided.", "")
		return
	}
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &HorizonProvider{
			version: version,
		}
	}
}
