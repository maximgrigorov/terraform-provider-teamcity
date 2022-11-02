package teamcity

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"terraform-provider-teamcity/client"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource              = &vcsRootResource{}
	_ resource.ResourceWithConfigure = &vcsRootResource{}
)

func NewVcsRootResource() resource.Resource {
	return &vcsRootResource{}
}

type vcsRootResource struct {
	client *client.Client
}

type vcsRootResourceModel struct {
	Name            types.String `tfsdk:"name"`
	Id              types.String `tfsdk:"id"`
	Type            types.String `tfsdk:"type"`
	PollingInterval types.Int64  `tfsdk:"polling_interval"`
	ProjectId       types.String `tfsdk:"project_id"`
	Url             types.String `tfsdk:"url"`
	Branch          types.String `tfsdk:"branch"`
}

func (r *vcsRootResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_vcsroot"
}

func (r *vcsRootResource) GetSchema(_ context.Context) (tfsdk.Schema, diag.Diagnostics) {
	return tfsdk.Schema{
		Attributes: map[string]tfsdk.Attribute{
			"name": {
				Type:     types.StringType,
				Required: true,
			},
			"id": {
				Type:     types.StringType,
				Optional: true,
				Computed: true,
			},
			"type": {
				Type:     types.StringType,
				Required: true,
			},
			"polling_interval": {
				Type:     types.Int64Type,
				Optional: true,
			},
			"project_id": {
				Type:     types.StringType,
				Required: true,
			},
			"url": {
				Type:     types.StringType,
				Required: true,
			},
			"branch": {
				Type:     types.StringType,
				Required: true,
			},
		},
	}, nil
}

func (r *vcsRootResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	r.client = req.ProviderData.(*client.Client)
}

func (r *vcsRootResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan vcsRootResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	interval := int(plan.PollingInterval.Value)
	root := client.VcsRoot{
		Name:            &plan.Name.Value,
		VcsName:         plan.Type.Value,
		PollingInterval: &interval,
		Project: client.ProjectLocator{
			Id: plan.ProjectId.Value,
		},
		Properties: client.VcsProperties{
			Property: []client.VcsProperty{
				{Name: "url", Value: plan.Url.Value},
				{Name: "branch", Value: plan.Branch.Value},
			},
		},
	}

	result, err := r.client.NewVcsRoot(root)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error setting VCS root",
			err.Error(),
		)
		return
	}

	read(result, &plan)

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r *vcsRootResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state vcsRootResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	actual, err := r.client.GetVcsRoot(state.Id.Value)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading VCS root",
			err.Error(),
		)
		return
	}

	read(actual, &state)

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func read(result *client.VcsRoot, plan *vcsRootResourceModel) {
	props := make(map[string]string)
	for _, p := range result.Properties.Property {
		props[p.Name] = p.Value
	}

	plan.Name = types.String{Value: *result.Name}
	plan.Id = types.String{Value: *result.Id}

	p := result.PollingInterval
	if p != nil {
		plan.PollingInterval = types.Int64{Value: int64(*result.PollingInterval)}
	} else {
		plan.PollingInterval = types.Int64{Null: true}
	}

	plan.Type = types.String{Value: result.VcsName}
	plan.ProjectId = types.String{Value: result.Project.Id}

	plan.Url = types.String{Value: props["url"]}
	plan.Branch = types.String{Value: props["branch"]}
}

type refType = func(*vcsRootResourceModel) any
type prop struct {
	ref      refType
	resource string
}

func (r *vcsRootResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan vcsRootResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state vcsRootResourceModel
	diags = req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	props := []prop{
		{
			ref:      func(a *vcsRootResourceModel) any { return &a.Name },
			resource: "name",
		},
		{
			ref:      func(a *vcsRootResourceModel) any { return &a.PollingInterval },
			resource: "modificationCheckInterval",
		},
		{
			ref:      func(a *vcsRootResourceModel) any { return &a.ProjectId },
			resource: "project",
		},
		{
			ref:      func(a *vcsRootResourceModel) any { return &a.Url },
			resource: "properties/url",
		},
		{
			ref:      func(a *vcsRootResourceModel) any { return &a.Branch },
			resource: "properties/branch",
		},
		{ // id is updated last
			ref:      func(a *vcsRootResourceModel) any { return &a.Id },
			resource: "id",
		},
	}

	for _, p := range props {
		err := r.setParameter(&plan, &state, p)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error setting VCS root field",
				err.Error(),
			)
			return
		}
	}

	if plan.Id.Unknown == true {
		plan.Id = types.String{Value: state.Id.Value}
	}

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r *vcsRootResource) setParameter(plan, state *vcsRootResourceModel, prop prop) error {
	switch param := prop.ref(plan).(type) {
	case *types.String:
		st := prop.ref(state).(*types.String)
		if param.Unknown != true && param.Value != st.Value {
			result, err := r.client.SetParameter(
				"vcs-roots",
				state.Id.Value,
				prop.resource,
				param.Value,
			)
			if err != nil {
				return err
			}
			param = &types.String{Value: *result}
		}
	case *types.Int64:
		st := prop.ref(state).(*types.Int64)
		if param.Unknown != true && param.Value != st.Value {
			var value string
			if param.IsNull() {
				value = ""
			} else {
				value = param.String()
			}
			result, err := r.client.SetParameter(
				"vcs-roots",
				state.Id.Value,
				prop.resource,
				value,
			)
			if err != nil {
				return err
			}

			i, err := strconv.ParseInt(*result, 10, 64)
			if err != nil {
				return err
			}
			param = &types.Int64{Value: i}
		}
	default:
		return errors.New("Unknown type: " + fmt.Sprintf("%T", param))
	}

	return nil
}

func (r *vcsRootResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state vcsRootResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeleteVcsRoot(state.Id.Value)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting VCS root",
			err.Error(),
		)
		return
	}
}
