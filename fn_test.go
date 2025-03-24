package main

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/samber/lo"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/crossplane/function-sdk-go/logging"
	fnv1 "github.com/crossplane/function-sdk-go/proto/v1"
	"github.com/crossplane/function-sdk-go/resource"
	"github.com/crossplane/function-sdk-go/response"
)

type KeyCloakMockClient struct {
}

func (c *KeyCloakMockClient) GetToken() (string, error) {
	return "1234", nil
}

func (c *KeyCloakMockClient) GetGroupMembers(groupName []string) ([]string, error) {
	if lo.Contains(groupName, "chuan") {
		return []string{"chuan@gmail.com", "hehe@gmail.com"}, nil
	}
	return nil, nil
}

func TestRunFunction(t *testing.T) {
	type args struct {
		ctx context.Context
		req *fnv1.RunFunctionRequest
	}
	type want struct {
		rsp *fnv1.RunFunctionResponse
		err error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"ResponseIsReturnedTypeFetchUser": {
			reason: "The Function should return a fatal result if no input was specified",
			args: args{
				req: &fnv1.RunFunctionRequest{
					Meta: &fnv1.RequestMeta{Tag: "hello"},
					Input: resource.MustStructJSON(`{
						"apiVersion": "template.fn.crossplane.io/v1beta1",
						"kind": "Input",
                        "groupList": {
							"fromCompositeField": "spec.adminOrgs"
						},
						"functionType": "FetchUser",
						"outputField": "spec.status.adminUsers"
					}`),
					Observed: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(`{
                                "apiVersion": "template.fn.crossplane.io/v1beta1",
                                "kind": "Output",
                                "spec": {
									"adminOrgs" : ["chuan", "hi"]
								}
                            }`),
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Tag: "hello", Ttl: durationpb.New(response.DefaultTTL)},
					Conditions: []*fnv1.Condition{
						{
							Type:   "FunctionSuccess",
							Status: fnv1.Status_STATUS_CONDITION_TRUE,
							Reason: "Success",
							Target: fnv1.Target_TARGET_COMPOSITE_AND_CLAIM.Enum(),
						},
					},
					Desired: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(`{
								"apiVersion": "template.fn.crossplane.io/v1beta1",
								"kind": "Output",
								"spec": {
									"status": {
										"adminUsers": ["chuan@gmail.com", "hehe@gmail.com"]
									}
								}
							}`),
						},
					},
				},
			},
		},
		"ResponseIsReturnedTypeDedupeUser": {
			reason: "The Function should return a fatal result if no input was specified",
			args: args{
				req: &fnv1.RunFunctionRequest{
					Meta: &fnv1.RequestMeta{Tag: "hello"},
					Input: resource.MustStructJSON(`{
						"apiVersion": "template.fn.crossplane.io/v1beta1",
						"kind": "Input",
						"functionType": "DedupeUsers",
						"groupsPriority": [
							{
								"fromPathsList": ["spec.adminUsers", "status.adminUsers"],
								"toPath": "spec.adminUsers"
							},
							{
								"fromPathsList": ["spec.editorUsers", "status.editorUsers"],
								"toPath": "spec.editorUsers"
							},
							{
								"fromPathsList": ["spec.viewerUsers", "status.viewerUsers"],
								"toPath": "spec.viewerUsers"
							}
						]
					}`),
					Observed: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(`{
                                "apiVersion": "template.fn.crossplane.io/v1beta1",
                                "kind": "Output",
                                "spec": {
									"adminUsers": ["chuan1@gmail.com", "chuan2@gmail.com"],
									"editorUsers": ["chuan1@gmail.com", "chuan2@gmail.com"],
									"viewerUsers": ["chuan1@gmail.com", "chuan2@gmail.com"]
								},
								"status": {
									"adminUsers": ["chuan1@gmail.com", "chuan3@gmail.com"],
									"editorUsers": ["chuan1@gmail.com", "chuan2@gmail.com"],
									"viewerUsers": ["chuan1@gmail.com", "chuan4@gmail.com"]
								}
                            }`),
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Tag: "hello", Ttl: durationpb.New(response.DefaultTTL)},
					Conditions: []*fnv1.Condition{
						{
							Type:   "FunctionSuccess",
							Status: fnv1.Status_STATUS_CONDITION_TRUE,
							Reason: "Success",
							Target: fnv1.Target_TARGET_COMPOSITE_AND_CLAIM.Enum(),
						},
					},
					Desired: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(`{
								"apiVersion": "template.fn.crossplane.io/v1beta1",
								"kind": "Output",
								"spec": {
									"adminUsers": ["chuan1@gmail.com", "chuan2@gmail.com","chuan3@gmail.com"],
									"viewerUsers": ["chuan4@gmail.com"]
								}
							}`),
						},
					},
				},
			},
		},
		"ResponseIsReturnedTypeDedupeUserCaseLackOfUser": {
			reason: "The Function should return a fatal result if no input was specified",
			args: args{
				req: &fnv1.RunFunctionRequest{
					Meta: &fnv1.RequestMeta{Tag: "hello"},
					Input: resource.MustStructJSON(`{
						"apiVersion": "template.fn.crossplane.io/v1beta1",
						"kind": "Input",
						"functionType": "DedupeUsers",
						"groupsPriority": [
							{
								"fromPathsList": ["spec.adminUsers", "status.adminUsers"],
								"toPath": "spec.adminUsers"
							},
							{
								"fromPathsList": ["spec.editorUsers", "status.editorUsers"],
								"toPath": "spec.editorUsers"
							},
							{
								"fromPathsList": ["spec.viewerUsers", "status.viewerUsers"],
								"toPath": "spec.viewerUsers"
							}
						]
					}`),
					Observed: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(`{
                                "apiVersion": "template.fn.crossplane.io/v1beta1",
                                "kind": "Output",
                                "spec": {
									"adminUsers": ["chuan1@gmail.com"],
									"viewerUsers": ["chuan1@gmail.com", "chuan4@gmail.com"]
								},
								"status": {
									"adminUsers": ["chuan1@gmail.com", "chuan2@gmail.com"],
									"editorUsers": ["chuan1@gmail.com", "chuan2@gmail.com"]
								}
                            }`),
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Tag: "hello", Ttl: durationpb.New(response.DefaultTTL)},
					Conditions: []*fnv1.Condition{
						{
							Type:   "FunctionSuccess",
							Status: fnv1.Status_STATUS_CONDITION_TRUE,
							Reason: "Success",
							Target: fnv1.Target_TARGET_COMPOSITE_AND_CLAIM.Enum(),
						},
					},
					Desired: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(`{
								"apiVersion": "template.fn.crossplane.io/v1beta1",
								"kind": "Output",
								"spec": {
									"adminUsers": ["chuan1@gmail.com", "chuan2@gmail.com"],
									"viewerUsers": ["chuan4@gmail.com"]
								}
							}`),
						},
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			keycloakClient := &KeyCloakMockClient{}
			f := &Function{log: logging.NewNopLogger(), keycloakClient: keycloakClient}
			rsp, err := f.RunFunction(tc.args.ctx, tc.args.req)
			less := func(a, b any) bool { return fmt.Sprintf("%s", a) < fmt.Sprintf("%s", b) }
			rsp.Results = nil
			if diff := cmp.Diff(tc.want.rsp, rsp, protocmp.Transform(), cmpopts.SortSlices(less)); diff != "" {
				t.Errorf("%s\nf.RunFunction(...): -want rsp, +got rsp:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("%s\nf.RunFunction(...): -want err, +got err:\n%s", tc.reason, diff)
			}
		})
	}
}
