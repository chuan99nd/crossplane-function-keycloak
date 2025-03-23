package main

import (
	"context"
	"fmt"

	"github.com/crossplane/function-keycloak/client"
	"github.com/crossplane/function-keycloak/input/v1beta1"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
	"github.com/crossplane/function-sdk-go"
	"github.com/crossplane/function-sdk-go/errors"
	"github.com/crossplane/function-sdk-go/logging"
	fnv1 "github.com/crossplane/function-sdk-go/proto/v1"
	"github.com/crossplane/function-sdk-go/request"
	"github.com/crossplane/function-sdk-go/response"
	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/runtime"
)

// Function returns whatever response you ask it to.
type Function struct {
	fnv1.UnimplementedFunctionRunnerServiceServer

	log            logging.Logger
	keycloakClient client.KeycloakClientInterface
}

func NewFunction(debug bool) (*Function, error) {
	log, err := function.NewLogger(debug)
	if err != nil {
		return nil, err
	}
	keyCloakClient := client.NewKeycloakClient()
	f := &Function{
		log:            log,
		keycloakClient: keyCloakClient,
	}
	return f, nil
}

// RunFunction runs the Function.
func (f *Function) RunFunction(_ context.Context, req *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {
	f.log.Info("Running function", "tag", req.GetMeta().GetTag())

	rsp := response.To(req, response.DefaultTTL)
	in := &v1beta1.Input{}
	if err := request.GetInput(req, in); err != nil {
		response.ConditionFalse(rsp, "FunctionSuccess", "InternalError").
			WithMessage("Something went wrong.").
			TargetCompositeAndClaim()
		response.Fatal(rsp, errors.Wrapf(err, "cannot get Function input from %T", req))
		return rsp, nil
	}

	resource := req.GetObserved().GetComposite().Resource
	convertedResource, err := runtime.DefaultUnstructuredConverter.ToUnstructured(resource)
	if err != nil {
		response.ConditionFalse(rsp, "FunctionSuccess", "InternalError").TargetComposite().WithMessage("Failed to convert resource to unstructured")
		response.Fatal(rsp, errors.Wrapf(err, fmt.Sprintf("cannot convert resource to unstructured %s", convertedResource)))
		return rsp, nil
	}

	groupList, err := fieldpath.Pave(convertedResource).GetValue(in.GroupList.FromCompositeField)
	if err != nil {
		response.ConditionFalse(rsp, "FunctionSuccess", "InternalError").TargetComposite().WithMessage("Failed to get group list from composite field")
		response.Fatal(rsp, errors.Wrapf(err, fmt.Sprintf("cannot get group list from composite field %s", in.GroupList.FromCompositeField)))
		return rsp, nil
	}

	groupsInterface := groupList.([]any)
	groups := lo.Map(groupsInterface, func(group any, index int) string {
		return group.(string)
	})

	userList, err := f.keycloakClient.GetGroupMembers(groups)
	if err != nil {
		response.ConditionFalse(rsp, "FunctionSuccess", "InternalError").TargetComposite().WithMessage("Failed to get list user")
		response.Fatal(rsp, errors.Wrapf(err, fmt.Sprintf("cannot get group user of group %s", groups)))
		return rsp, nil
	}

	outPath := in.OutputField
	dxr, err := request.GetDesiredCompositeResource(req)
	if err != nil {
		response.ConditionFalse(rsp, "FunctionSuccess", "InternalError").TargetComposite().WithMessage("Failed to get DXR")
		response.Fatal(rsp, errors.Wrapf(err, fmt.Sprintf("Failed to get DXR")))
		return rsp, nil
	}

	// This is a bit of a hack. The Functions spec tells us we should only
	// return the desired status of the XR. Crossplane doesn't need anything
	// else. It already knows the XR's GVK and name, and thus "re-injects" them
	// into the desired state before applying it. However we need a GVK to be
	// able to use runtime.DefaultUnstructuredConverter internally, which fails
	// if you ask it to unmarshal JSON/YAML without a kind. Technically the
	// Function spec doesn't say anything about APIVersion and Kind, so we can
	// return these without being in violation. ;)
	// https://github.com/crossplane/crossplane/blob/53f71/contributing/specifications/functions.md
	oxr, err := request.GetObservedCompositeResource(req)
	if err != nil {
		response.Fatal(rsp, errors.Wrap(err, "cannot get observed composite resource"))
		return rsp, nil
	}
	dxr.Resource.SetAPIVersion(oxr.Resource.GetAPIVersion())
	dxr.Resource.SetKind(oxr.Resource.GetKind())

	err = patchFieldValueToObject(outPath, userList, dxr.Resource, nil)
	fmt.Println(dxr.Resource.Object)
	if err != nil {
		response.ConditionFalse(rsp, "FunctionSuccess", "InternalError").TargetComposite().WithMessage("Failed to get patch user to composite")
		response.Fatal(rsp, errors.Wrapf(err, "failed to patch user to DXR"))
		return rsp, nil
	}

	if err = response.SetDesiredCompositeResource(rsp, dxr); err != nil {
		response.Fatal(rsp, errors.Wrapf(err, "cannot set desired composite resource in %T", rsp))
		return rsp, nil
	}

	response.ConditionTrue(rsp, "FunctionSuccess", "Success").
		TargetCompositeAndClaim()

	return rsp, nil
}

func patchFieldValueToObject(fieldPath string, value any, to runtime.Object, mo *xpv1.MergeOptions) error {
	paved, err := fieldpath.PaveObject(to)
	if err != nil {
		return err
	}

	if err := paved.MergeValue(fieldPath, value, mo); err != nil {
		return err
	}

	return runtime.DefaultUnstructuredConverter.FromUnstructured(paved.UnstructuredContent(), to)
}
