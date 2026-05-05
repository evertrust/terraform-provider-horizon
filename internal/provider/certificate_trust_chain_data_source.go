package provider

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	horizon "github.com/evertrust/horizon-go/v2"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

const (
	trustChainOrderLeafToRoot        = "leaf_to_root"
	trustChainOrderRootToLeaf        = "root_to_leaf"
	trustChainOrderIssuerLeafToRoot  = "issuer_leaf_to_root"
	trustChainOrderIssuerRootToLeaf  = "issuer_root_to_leaf"
)

// orderToHorizon maps the provider-side order values to the Horizon API values.
var orderToHorizon = map[string]string{
	trustChainOrderLeafToRoot:       "ltr",
	trustChainOrderRootToLeaf:       "rtl",
	trustChainOrderIssuerLeafToRoot: "iltr",
	trustChainOrderIssuerRootToLeaf: "irtl",
}

func NewCertificateTrustChainDataSource() datasource.DataSource {
	return &CertificateTrustChainDataSource{}
}

type CertificateTrustChainDataSource struct {
	client *horizon.APIClient
}

type certificateTrustChainDataSourceModel struct {
	Id             types.String `tfsdk:"id"`
	CertificatePem types.String `tfsdk:"certificate_pem"`
	Order          types.String `tfsdk:"order"`
	Chain          types.List   `tfsdk:"chain"`
	ChainPem       types.String `tfsdk:"chain_pem"`
	Length         types.Int64  `tfsdk:"length"`
}

func (d *CertificateTrustChainDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_certificate_trust_chain"
}

func (d *CertificateTrustChainDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetches the trust chain of a PEM-encoded X.509 certificate from Horizon. The chain order returned by Horizon is preserved as-is.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Stable identifier derived from the input certificate content and the requested order.",
			},
			"certificate_pem": schema.StringAttribute{
				Required:    true,
				Description: "PEM-encoded X.509 certificate to look up the trust chain for.",
			},
			"order": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Order of the returned chain. One of `leaf_to_root`, `root_to_leaf`, `issuer_leaf_to_root`, `issuer_root_to_leaf`. Defaults to `leaf_to_root`.",
			},
			"chain": schema.ListAttribute{
				Computed:    true,
				ElementType: types.StringType,
				Description: "PEM-encoded certificates returned by Horizon, one entry per certificate, in the requested order.",
			},
			"chain_pem": schema.StringAttribute{
				Computed:    true,
				Description: "Concatenated PEM bundle of the trust chain, in the requested order.",
			},
			"length": schema.Int64Attribute{
				Computed:    true,
				Description: "Number of certificates in the returned chain.",
			},
		},
	}
}

func (d *CertificateTrustChainDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*horizon.APIClient)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *horizon.APIClient, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	d.client = client
}

func (d *CertificateTrustChainDataSource) ValidateConfig(ctx context.Context, req datasource.ValidateConfigRequest, resp *datasource.ValidateConfigResponse) {
	var data certificateTrustChainDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !data.Order.IsNull() && !data.Order.IsUnknown() {
		if _, ok := orderToHorizon[data.Order.ValueString()]; !ok {
			resp.Diagnostics.AddAttributeError(
				path.Root("order"),
				"Invalid order value",
				fmt.Sprintf("order must be one of %q, %q, %q, or %q.",
					trustChainOrderLeafToRoot,
					trustChainOrderRootToLeaf,
					trustChainOrderIssuerLeafToRoot,
					trustChainOrderIssuerRootToLeaf),
			)
		}
	}

	if !data.CertificatePem.IsNull() && !data.CertificatePem.IsUnknown() {
		if strings.TrimSpace(data.CertificatePem.ValueString()) == "" {
			resp.Diagnostics.AddAttributeError(
				path.Root("certificate_pem"),
				"certificate_pem must not be empty",
				"Provide a PEM-encoded X.509 certificate.",
			)
		}
	}
}

func (d *CertificateTrustChainDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data certificateTrustChainDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	order := trustChainOrderLeafToRoot
	if !data.Order.IsNull() && !data.Order.IsUnknown() && data.Order.ValueString() != "" {
		order = data.Order.ValueString()
	}

	horizonOrder, ok := orderToHorizon[order]
	if !ok {
		resp.Diagnostics.AddAttributeError(
			path.Root("order"),
			"Invalid order value",
			fmt.Sprintf("order must be one of %q, %q, %q, or %q.",
				trustChainOrderLeafToRoot,
				trustChainOrderRootToLeaf,
				trustChainOrderIssuerLeafToRoot,
				trustChainOrderIssuerRootToLeaf),
		)
		return
	}

	pem := data.CertificatePem.ValueString()
	if strings.TrimSpace(pem) == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("certificate_pem"),
			"certificate_pem must not be empty",
			"Provide a PEM-encoded X.509 certificate.",
		)
		return
	}

	chainResp, _, err := d.client.Rfc5280API.Rfc5280TcPem(ctx, pem).
		Order(horizonOrder).
		Execute()
	if err != nil {
		resp.Diagnostics.AddError("Failed to retrieve certificate trust chain", err.Error())
		return
	}

	if len(chainResp) == 0 {
		resp.Diagnostics.AddError(
			"Empty trust chain",
			"Horizon returned an empty trust chain for the provided certificate.",
		)
		return
	}

	chainPems := make([]string, 0, len(chainResp))
	for i, c := range chainResp {
		if c.Pem == "" {
			resp.Diagnostics.AddError(
				"Missing PEM in trust chain entry",
				fmt.Sprintf("Horizon returned a trust chain entry without a PEM at index %d.", i),
			)
			return
		}
		chainPems = append(chainPems, c.Pem)
	}

	chainList, diags := types.ListValueFrom(ctx, types.StringType, chainPems)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	data.Chain = chainList
	data.ChainPem = types.StringValue(strings.Join(chainPems, "\n"))
	data.Length = types.Int64Value(int64(len(chainPems)))
	data.Order = types.StringValue(order)
	data.Id = types.StringValue(trustChainID(pem, order))

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// trustChainID returns a deterministic identifier derived from the certificate
// content and the requested order, so Terraform sees a stable id across reads.
func trustChainID(pem, order string) string {
	h := sha256.New()
	h.Write([]byte(order))
	h.Write([]byte{0})
	h.Write([]byte(pem))
	return hex.EncodeToString(h.Sum(nil))
}
