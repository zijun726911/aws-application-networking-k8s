package gateway

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/vpclattice"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/pointer"
	testclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	gwv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
	gwv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
	mcsv1alpha1 "sigs.k8s.io/mcs-api/pkg/apis/v1alpha1"

	"github.com/aws/aws-application-networking-k8s/pkg/k8s"
	"github.com/aws/aws-application-networking-k8s/pkg/model/core"
	model "github.com/aws/aws-application-networking-k8s/pkg/model/lattice"
	"github.com/aws/aws-application-networking-k8s/pkg/utils/gwlog"
)

type dummyTgBuilder struct {
	i int
}

func (d *dummyTgBuilder) Build(ctx context.Context, route core.Route, backendRef core.BackendRef, stack core.Stack) (core.Stack, *model.TargetGroup, error) {
	// just need to provide a TG with an ID
	id := fmt.Sprintf("tg-%d", d.i)
	d.i++

	tg := &model.TargetGroup{
		ResourceMeta: core.NewResourceMeta(stack, "AWS:VPCServiceNetwork::TargetGroup", id),
	}
	stack.AddResource(tg)
	return stack, tg, nil
}

func Test_RuleModelBuild(t *testing.T) {
	var httpSectionName gwv1beta1.SectionName = "http"
	var serviceKind gwv1beta1.Kind = "Service"
	var serviceImportKind gwv1beta1.Kind = "ServiceImport"
	var weight1 = int32(10)
	var weight2 = int32(90)
	var namespace = gwv1beta1.Namespace("testnamespace")
	var namespace2 = gwv1beta1.Namespace("testnamespace2")
	var path1 = "/ver1"
	var path2 = "/ver2"
	var path3 = "/ver3"
	var httpGet = gwv1beta1.HTTPMethodGet
	var httpPost = gwv1beta1.HTTPMethodPost
	var k8sPathMatchExactType = gwv1beta1.PathMatchExact
	var k8sPathMatchPrefix = gwv1beta1.PathMatchPathPrefix
	var k8sMethodMatchExactType = gwv1alpha2.GRPCMethodMatchExact
	var k8sHeaderExactType = gwv1beta1.HeaderMatchExact
	var k8sHeaderRegexType = gwv1beta1.HeaderMatchRegularExpression
	var hdr1 = "env1"
	var hdr1Value = "test1"
	var hdr2 = "env2"
	var hdr2Value = "test2"

	var backendRef1 = gwv1beta1.BackendRef{
		BackendObjectReference: gwv1beta1.BackendObjectReference{
			Name: "targetgroup1",
			Kind: &serviceKind,
		},
		Weight: &weight1,
	}
	var backendRef2 = gwv1beta1.BackendRef{
		BackendObjectReference: gwv1beta1.BackendObjectReference{
			Name: "targetgroup2",
			Kind: &serviceImportKind,
		},
		Weight: &weight2,
	}
	var backendRef1Namespace1 = gwv1beta1.BackendRef{
		BackendObjectReference: gwv1beta1.BackendObjectReference{
			Name:      "targetgroup2",
			Namespace: &namespace,
			Kind:      &serviceImportKind,
		},
		Weight: &weight2,
	}
	var backendRef1Namespace2 = gwv1beta1.BackendRef{
		BackendObjectReference: gwv1beta1.BackendObjectReference{
			Name:      "targetgroup2",
			Namespace: &namespace2,
			Kind:      &serviceImportKind,
		},
		Weight: &weight2,
	}
	var backendServiceImportRef = gwv1beta1.BackendRef{
		BackendObjectReference: gwv1beta1.BackendObjectReference{
			Name: "targetgroup1",
			Kind: &serviceImportKind,
		},
	}

	tests := []struct {
		name         string
		route        core.Route
		wantErrIsNil bool
		expectedSpec []model.RuleSpec
	}{
		{
			name:         "rule, default service action",
			wantErrIsNil: true,
			route: core.NewHTTPRoute(gwv1beta1.HTTPRoute{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "service1",
					Namespace: "default",
				},
				Spec: gwv1beta1.HTTPRouteSpec{
					CommonRouteSpec: gwv1beta1.CommonRouteSpec{
						ParentRefs: []gwv1beta1.ParentReference{
							{
								Name:        "gw1",
								SectionName: &httpSectionName,
							},
						},
					},
					Rules: []gwv1beta1.HTTPRouteRule{
						{
							BackendRefs: []gwv1beta1.HTTPBackendRef{
								{
									BackendRef: backendRef1,
								},
							},
						},
					},
				},
			}),
			expectedSpec: []model.RuleSpec{ // note priority is only calculated at synthesis b/c it requires access to existing rules
				{
					StackListenerId: "listener-id",
					Action: model.RuleAction{
						TargetGroups: []*model.RuleTargetGroup{
							{
								StackTargetGroupId: "tg-0",
								Weight:             int64(weight1),
							},
						},
					},
				},
			},
		},
		{
			name:         "rule, default serviceimport action",
			wantErrIsNil: true,
			route: core.NewHTTPRoute(gwv1beta1.HTTPRoute{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "service1",
					Namespace: "default",
				},
				Spec: gwv1beta1.HTTPRouteSpec{
					CommonRouteSpec: gwv1beta1.CommonRouteSpec{
						ParentRefs: []gwv1beta1.ParentReference{
							{
								Name:        "gw1",
								SectionName: &httpSectionName,
							},
						},
					},
					Rules: []gwv1beta1.HTTPRouteRule{
						{
							BackendRefs: []gwv1beta1.HTTPBackendRef{
								{
									BackendRef: backendServiceImportRef,
								},
							},
						},
					},
				},
			}),
			expectedSpec: []model.RuleSpec{
				{
					StackListenerId: "listener-id",
					Action: model.RuleAction{
						TargetGroups: []*model.RuleTargetGroup{
							{
								SvcImportTG: &model.SvcImportTargetGroup{
									K8SServiceName:      string(backendServiceImportRef.Name),
									K8SServiceNamespace: "default",
								},
								Weight: 1,
							},
						},
					},
				},
			},
		},
		{
			name:         "rule, weighted target group",
			wantErrIsNil: true,
			route: core.NewHTTPRoute(gwv1beta1.HTTPRoute{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "service1",
					Namespace: "default",
				},
				Spec: gwv1beta1.HTTPRouteSpec{
					CommonRouteSpec: gwv1beta1.CommonRouteSpec{
						ParentRefs: []gwv1beta1.ParentReference{
							{
								Name:        "gw1",
								SectionName: &httpSectionName,
							},
						},
					},
					Rules: []gwv1beta1.HTTPRouteRule{
						{
							BackendRefs: []gwv1beta1.HTTPBackendRef{
								{
									BackendRef: backendRef1,
								},
								{
									BackendRef: backendRef2,
								},
							},
						},
					},
				},
			}),
			expectedSpec: []model.RuleSpec{
				{
					StackListenerId: "listener-id",
					Action: model.RuleAction{
						TargetGroups: []*model.RuleTargetGroup{
							{
								StackTargetGroupId: "tg-0",
								Weight:             int64(weight1),
							},
							{
								SvcImportTG: &model.SvcImportTargetGroup{
									K8SServiceName:      string(backendRef2.Name),
									K8SServiceNamespace: "default",
								},
								Weight: int64(weight2),
							},
						},
					},
				},
			},
		},
		{
			name:         "rule, path based target group",
			wantErrIsNil: true,
			route: core.NewHTTPRoute(gwv1beta1.HTTPRoute{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "service1",
					Namespace: "default",
				},
				Spec: gwv1beta1.HTTPRouteSpec{
					CommonRouteSpec: gwv1beta1.CommonRouteSpec{
						ParentRefs: []gwv1beta1.ParentReference{
							{
								Name:        "gw1",
								SectionName: &httpSectionName,
							},
						},
					},
					Rules: []gwv1beta1.HTTPRouteRule{
						{
							Matches: []gwv1beta1.HTTPRouteMatch{
								{
									Path: &gwv1beta1.HTTPPathMatch{
										Type:  &k8sPathMatchExactType,
										Value: &path1,
									},
								},
							},
							BackendRefs: []gwv1beta1.HTTPBackendRef{
								{
									BackendRef: backendRef1,
								},
							},
						},
						{
							Matches: []gwv1beta1.HTTPRouteMatch{
								{
									Path: &gwv1beta1.HTTPPathMatch{
										Type:  &k8sPathMatchPrefix,
										Value: &path2,
									},
								},
							},
							BackendRefs: []gwv1beta1.HTTPBackendRef{
								{
									BackendRef: backendRef2,
								},
							},
						},
					},
				},
			}),
			expectedSpec: []model.RuleSpec{
				{
					StackListenerId: "listener-id",
					PathMatchExact:  true,
					PathMatchValue:  path1,
					Action: model.RuleAction{
						TargetGroups: []*model.RuleTargetGroup{
							{
								StackTargetGroupId: "tg-0",
								Weight:             int64(weight1),
							},
						},
					},
				},
				{
					StackListenerId: "listener-id",
					PathMatchPrefix: true,
					PathMatchValue:  path2,
					Action: model.RuleAction{
						TargetGroups: []*model.RuleTargetGroup{
							{
								SvcImportTG: &model.SvcImportTargetGroup{
									K8SServiceName:      string(backendRef2.Name),
									K8SServiceNamespace: "default",
								},
								Weight: int64(weight2),
							},
						},
					},
				},
			},
		},
		{
			name:         "rule, method based",
			wantErrIsNil: true,
			route: core.NewHTTPRoute(gwv1beta1.HTTPRoute{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "service1",
					Namespace: "default",
				},
				Spec: gwv1beta1.HTTPRouteSpec{
					CommonRouteSpec: gwv1beta1.CommonRouteSpec{
						ParentRefs: []gwv1beta1.ParentReference{
							{
								Name:        "gw1",
								SectionName: &httpSectionName,
							},
						},
					},
					Rules: []gwv1beta1.HTTPRouteRule{
						{
							Matches: []gwv1beta1.HTTPRouteMatch{
								{
									Method: &httpGet,
								},
							},
							BackendRefs: []gwv1beta1.HTTPBackendRef{
								{
									BackendRef: backendRef1,
								},
							},
						},
						{
							Matches: []gwv1beta1.HTTPRouteMatch{
								{
									Method: &httpPost,
								},
							},
							BackendRefs: []gwv1beta1.HTTPBackendRef{
								{
									BackendRef: backendRef2,
								},
							},
						},
					},
				},
			}),
			expectedSpec: []model.RuleSpec{
				{
					StackListenerId: "listener-id",
					Method:          string(httpGet),
					Action: model.RuleAction{
						TargetGroups: []*model.RuleTargetGroup{
							{
								StackTargetGroupId: "tg-0",
								Weight:             int64(weight1),
							},
						},
					},
				},
				{
					StackListenerId: "listener-id",
					Method:          string(httpPost),
					Action: model.RuleAction{
						TargetGroups: []*model.RuleTargetGroup{
							{
								SvcImportTG: &model.SvcImportTargetGroup{
									K8SServiceName:      string(backendRef2.Name),
									K8SServiceNamespace: "default",
								},
								Weight: int64(weight2),
							},
						},
					},
				},
			},
		},
		{
			name:         "rule, different namespace combination",
			wantErrIsNil: true,
			route: core.NewHTTPRoute(gwv1beta1.HTTPRoute{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "service1",
					Namespace: "non-default",
				},
				Spec: gwv1beta1.HTTPRouteSpec{
					CommonRouteSpec: gwv1beta1.CommonRouteSpec{
						ParentRefs: []gwv1beta1.ParentReference{
							{
								Name:        "gw1",
								SectionName: &httpSectionName,
							},
						},
					},
					Rules: []gwv1beta1.HTTPRouteRule{
						{
							Matches: []gwv1beta1.HTTPRouteMatch{
								{
									Path: &gwv1beta1.HTTPPathMatch{
										Value: &path1,
										Type:  &k8sPathMatchExactType,
									},
								},
							},
							BackendRefs: []gwv1beta1.HTTPBackendRef{
								{
									BackendRef: backendRef1,
								},
							},
						},
						{
							Matches: []gwv1beta1.HTTPRouteMatch{
								{
									Path: &gwv1beta1.HTTPPathMatch{
										Value: &path2,
										Type:  &k8sPathMatchExactType,
									},
								},
							},
							BackendRefs: []gwv1beta1.HTTPBackendRef{
								{
									BackendRef: backendRef1Namespace1,
								},
							},
						},
						{
							Matches: []gwv1beta1.HTTPRouteMatch{
								{
									Path: &gwv1beta1.HTTPPathMatch{
										Value: &path3,
										Type:  &k8sPathMatchExactType,
									},
								},
							},
							BackendRefs: []gwv1beta1.HTTPBackendRef{
								{
									BackendRef: backendRef1Namespace2,
								},
							},
						},
					},
				},
			}),
			expectedSpec: []model.RuleSpec{
				{
					StackListenerId: "listener-id",
					PathMatchExact:  true,
					PathMatchValue:  path1,
					Action: model.RuleAction{
						TargetGroups: []*model.RuleTargetGroup{
							{
								StackTargetGroupId: "tg-0",
								Weight:             int64(weight1),
							},
						},
					},
				},
				{
					StackListenerId: "listener-id",
					PathMatchExact:  true,
					PathMatchValue:  path2,
					Action: model.RuleAction{
						TargetGroups: []*model.RuleTargetGroup{
							{
								SvcImportTG: &model.SvcImportTargetGroup{
									K8SServiceName:      string(backendRef1Namespace1.Name),
									K8SServiceNamespace: string(*backendRef1Namespace1.Namespace),
								},
								Weight: int64(weight2),
							},
						},
					},
				},
				{
					StackListenerId: "listener-id",
					PathMatchExact:  true,
					PathMatchValue:  path3,
					Action: model.RuleAction{
						TargetGroups: []*model.RuleTargetGroup{
							{
								SvcImportTG: &model.SvcImportTargetGroup{
									K8SServiceName:      string(backendRef1Namespace2.Name),
									K8SServiceNamespace: string(*backendRef1Namespace2.Namespace),
								},
								Weight: int64(weight2),
							},
						},
					},
				},
			},
		},
		{
			name:         "rule, default service import action for GRPCRoute",
			wantErrIsNil: true,
			route: core.NewGRPCRoute(gwv1alpha2.GRPCRoute{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "service1",
					Namespace: "default",
				},
				Spec: gwv1alpha2.GRPCRouteSpec{
					CommonRouteSpec: gwv1alpha2.CommonRouteSpec{
						ParentRefs: []gwv1alpha2.ParentReference{
							{
								Name:        "gw1",
								SectionName: &httpSectionName,
							},
						},
					},
					Rules: []gwv1alpha2.GRPCRouteRule{
						{
							BackendRefs: []gwv1alpha2.GRPCBackendRef{
								{
									BackendRef: backendServiceImportRef,
								},
							},
						},
					},
				},
			}),
			expectedSpec: []model.RuleSpec{
				{
					StackListenerId: "listener-id",
					Action: model.RuleAction{
						TargetGroups: []*model.RuleTargetGroup{
							{
								SvcImportTG: &model.SvcImportTargetGroup{
									K8SServiceName:      string(backendServiceImportRef.Name),
									K8SServiceNamespace: "default",
								},
								Weight: 1,
							},
						},
					},
				},
			},
		},
		{
			name:         "rule, gRPC routes with methods and multiple namespaces",
			wantErrIsNil: true,
			route: core.NewGRPCRoute(gwv1alpha2.GRPCRoute{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "service1",
					Namespace: "non-default",
				},
				Spec: gwv1alpha2.GRPCRouteSpec{
					CommonRouteSpec: gwv1beta1.CommonRouteSpec{
						ParentRefs: []gwv1beta1.ParentReference{
							{
								Name:        "gw1",
								SectionName: &httpSectionName,
							},
						},
					},
					Rules: []gwv1alpha2.GRPCRouteRule{
						{
							Matches: []gwv1alpha2.GRPCRouteMatch{
								{
									Method: &gwv1alpha2.GRPCMethodMatch{
										Type:    &k8sMethodMatchExactType,
										Service: pointer.String("service"),
										Method:  pointer.String("method1"),
									},
								},
							},
							BackendRefs: []gwv1alpha2.GRPCBackendRef{
								{
									BackendRef: backendRef1,
								},
							},
						},
						{
							Matches: []gwv1alpha2.GRPCRouteMatch{
								{
									Method: &gwv1alpha2.GRPCMethodMatch{
										Type:    &k8sMethodMatchExactType,
										Service: pointer.String("service"),
										Method:  pointer.String("method2"),
									},
								},
							},
							BackendRefs: []gwv1alpha2.GRPCBackendRef{
								{
									BackendRef: backendRef1Namespace1,
								},
							},
						},
						{
							Matches: []gwv1alpha2.GRPCRouteMatch{
								{
									Method: &gwv1alpha2.GRPCMethodMatch{
										Type:    &k8sMethodMatchExactType,
										Service: pointer.String("service"),
										Method:  pointer.String("method3"),
									},
								},
							},
							BackendRefs: []gwv1alpha2.GRPCBackendRef{
								{
									BackendRef: backendRef1Namespace2,
								},
							},
						},
					},
				},
			}),
			expectedSpec: []model.RuleSpec{
				{
					StackListenerId: "listener-id",
					PathMatchExact:  true,
					PathMatchValue:  "/service/method1",
					Method:          string(httpPost),
					Action: model.RuleAction{
						TargetGroups: []*model.RuleTargetGroup{
							{
								StackTargetGroupId: "tg-0",
								Weight:             int64(weight1),
							},
						},
					},
				},
				{
					StackListenerId: "listener-id",
					PathMatchExact:  true,
					PathMatchValue:  "/service/method2",
					Method:          string(httpPost),
					Action: model.RuleAction{
						TargetGroups: []*model.RuleTargetGroup{
							{
								SvcImportTG: &model.SvcImportTargetGroup{
									K8SServiceName:      string(backendRef1Namespace1.Name),
									K8SServiceNamespace: string(*backendRef1Namespace1.Namespace),
								},
								Weight: int64(weight2),
							},
						},
					},
				},
				{
					StackListenerId: "listener-id",
					PathMatchExact:  true,
					PathMatchValue:  "/service/method3",
					Method:          string(httpPost),
					Action: model.RuleAction{
						TargetGroups: []*model.RuleTargetGroup{
							{
								SvcImportTG: &model.SvcImportTargetGroup{
									K8SServiceName:      string(backendRef1Namespace2.Name),
									K8SServiceNamespace: string(*backendRef1Namespace2.Namespace),
								},
								Weight: int64(weight2),
							},
						},
					},
				},
			},
		},
		{
			name:         "1 header match",
			wantErrIsNil: true,
			route: core.NewHTTPRoute(gwv1beta1.HTTPRoute{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "service1",
					Namespace: "default",
				},
				Spec: gwv1beta1.HTTPRouteSpec{
					CommonRouteSpec: gwv1beta1.CommonRouteSpec{
						ParentRefs: []gwv1beta1.ParentReference{
							{
								Name:        "gw1",
								SectionName: &httpSectionName,
							},
						},
					},
					Rules: []gwv1beta1.HTTPRouteRule{
						{
							Matches: []gwv1beta1.HTTPRouteMatch{
								{
									Headers: []gwv1beta1.HTTPHeaderMatch{
										{
											Type:  &k8sHeaderExactType,
											Name:  gwv1beta1.HTTPHeaderName(hdr1),
											Value: hdr1Value,
										},
									},
								},
							},
							BackendRefs: []gwv1beta1.HTTPBackendRef{
								{
									BackendRef: backendRef1,
								},
							},
						},
					},
				},
			}),
			expectedSpec: []model.RuleSpec{
				{
					StackListenerId: "listener-id",
					MatchedHeaders: []vpclattice.HeaderMatch{
						{
							Name: &hdr1,
							Match: &vpclattice.HeaderMatchType{
								Exact: &hdr1Value,
							},
							CaseSensitive: aws.Bool(false),
						},
					},
					Action: model.RuleAction{
						TargetGroups: []*model.RuleTargetGroup{
							{
								StackTargetGroupId: "tg-0",
								Weight:             int64(*backendRef1.Weight),
							},
						},
					},
				},
			},
		},
		{
			name:         "2 header match",
			wantErrIsNil: true,
			route: core.NewHTTPRoute(gwv1beta1.HTTPRoute{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "service1",
					Namespace: "default",
				},
				Spec: gwv1beta1.HTTPRouteSpec{
					CommonRouteSpec: gwv1beta1.CommonRouteSpec{
						ParentRefs: []gwv1beta1.ParentReference{
							{
								Name:        "gw1",
								SectionName: &httpSectionName,
							},
						},
					},
					Rules: []gwv1beta1.HTTPRouteRule{
						{
							Matches: []gwv1beta1.HTTPRouteMatch{
								{
									Headers: []gwv1beta1.HTTPHeaderMatch{
										{
											Type:  &k8sHeaderExactType,
											Name:  gwv1beta1.HTTPHeaderName(hdr1),
											Value: hdr1Value,
										},
										{
											Type:  &k8sHeaderExactType,
											Name:  gwv1beta1.HTTPHeaderName(hdr2),
											Value: hdr2Value,
										},
									},
								},
							},
							BackendRefs: []gwv1beta1.HTTPBackendRef{
								{
									BackendRef: backendRef1,
								},
							},
						},
					},
				},
			}),
			expectedSpec: []model.RuleSpec{
				{
					StackListenerId: "listener-id",
					MatchedHeaders: []vpclattice.HeaderMatch{
						{
							Name: &hdr1,
							Match: &vpclattice.HeaderMatchType{
								Exact: &hdr1Value,
							},
							CaseSensitive: aws.Bool(false),
						},
						{
							Name: &hdr2,
							Match: &vpclattice.HeaderMatchType{
								Exact: &hdr2Value,
							},
							CaseSensitive: aws.Bool(false),
						},
					},
					Action: model.RuleAction{
						TargetGroups: []*model.RuleTargetGroup{
							{
								StackTargetGroupId: "tg-0",
								Weight:             int64(*backendRef1.Weight),
							},
						},
					},
				},
			},
		},
		{
			name:         "2 header match with path exact",
			wantErrIsNil: true,
			route: core.NewHTTPRoute(gwv1beta1.HTTPRoute{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "service1",
					Namespace: "default",
				},
				Spec: gwv1beta1.HTTPRouteSpec{
					CommonRouteSpec: gwv1beta1.CommonRouteSpec{
						ParentRefs: []gwv1beta1.ParentReference{
							{
								Name:        "gw1",
								SectionName: &httpSectionName,
							},
						},
					},
					Rules: []gwv1beta1.HTTPRouteRule{
						{
							Matches: []gwv1beta1.HTTPRouteMatch{
								{

									Path: &gwv1beta1.HTTPPathMatch{
										Type:  &k8sPathMatchExactType,
										Value: &path1,
									},
									Headers: []gwv1beta1.HTTPHeaderMatch{
										{
											Type:  &k8sHeaderExactType,
											Name:  gwv1beta1.HTTPHeaderName(hdr1),
											Value: hdr1Value,
										},
										{
											Type:  &k8sHeaderExactType,
											Name:  gwv1beta1.HTTPHeaderName(hdr2),
											Value: hdr2Value,
										},
									},
								},
							},
							BackendRefs: []gwv1beta1.HTTPBackendRef{
								{
									BackendRef: backendRef1,
								},
							},
						},
					},
				},
			}),
			expectedSpec: []model.RuleSpec{
				{
					StackListenerId: "listener-id",
					PathMatchExact:  true,
					PathMatchValue:  path1,
					MatchedHeaders: []vpclattice.HeaderMatch{
						{
							Name: &hdr1,
							Match: &vpclattice.HeaderMatchType{
								Exact: &hdr1Value,
							},
							CaseSensitive: aws.Bool(false),
						},
						{
							Name: &hdr2,
							Match: &vpclattice.HeaderMatchType{
								Exact: &hdr2Value,
							},
							CaseSensitive: aws.Bool(false),
						},
					},
					Action: model.RuleAction{
						TargetGroups: []*model.RuleTargetGroup{
							{
								StackTargetGroupId: "tg-0",
								Weight:             int64(*backendRef1.Weight),
							},
						},
					},
				},
			},
		},
		{
			name:         "2 header match with path prefix",
			wantErrIsNil: true,
			route: core.NewHTTPRoute(gwv1beta1.HTTPRoute{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "service1",
					Namespace: "default",
				},
				Spec: gwv1beta1.HTTPRouteSpec{
					CommonRouteSpec: gwv1beta1.CommonRouteSpec{
						ParentRefs: []gwv1beta1.ParentReference{
							{
								Name:        "gw1",
								SectionName: &httpSectionName,
							},
						},
					},
					Rules: []gwv1beta1.HTTPRouteRule{
						{
							Matches: []gwv1beta1.HTTPRouteMatch{
								{

									Path: &gwv1beta1.HTTPPathMatch{
										Type:  &k8sPathMatchPrefix,
										Value: &path1,
									},
									Headers: []gwv1beta1.HTTPHeaderMatch{
										{
											Type:  &k8sHeaderExactType,
											Name:  gwv1beta1.HTTPHeaderName(hdr1),
											Value: hdr1Value,
										},
										{
											Type:  &k8sHeaderExactType,
											Name:  gwv1beta1.HTTPHeaderName(hdr2),
											Value: hdr2Value,
										},
									},
								},
							},
							BackendRefs: []gwv1beta1.HTTPBackendRef{
								{
									BackendRef: backendRef1,
								},
							},
						},
					},
				},
			}),
			expectedSpec: []model.RuleSpec{
				{
					StackListenerId: "listener-id",
					PathMatchPrefix: true,
					PathMatchValue:  path1,
					MatchedHeaders: []vpclattice.HeaderMatch{
						{
							Name: &hdr1,
							Match: &vpclattice.HeaderMatchType{
								Exact: &hdr1Value,
							},
							CaseSensitive: aws.Bool(false),
						},
						{
							Name: &hdr2,
							Match: &vpclattice.HeaderMatchType{
								Exact: &hdr2Value,
							},
							CaseSensitive: aws.Bool(false),
						},
					},
					Action: model.RuleAction{
						TargetGroups: []*model.RuleTargetGroup{
							{
								StackTargetGroupId: "tg-0",
								Weight:             int64(*backendRef1.Weight),
							},
						},
					},
				},
			},
		},
		{
			name:         "1 exact and 4 regular expression header matches",
			wantErrIsNil: true,
			route: core.NewHTTPRoute(gwv1beta1.HTTPRoute{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "service1",
					Namespace: "default",
				},
				Spec: gwv1beta1.HTTPRouteSpec{
					CommonRouteSpec: gwv1beta1.CommonRouteSpec{
						ParentRefs: []gwv1beta1.ParentReference{
							{
								Name:        "gw1",
								SectionName: &httpSectionName,
							},
						},
					},
					Rules: []gwv1beta1.HTTPRouteRule{
						{
							Matches: []gwv1beta1.HTTPRouteMatch{
								{
									Headers: []gwv1beta1.HTTPHeaderMatch{
										{
											Type:  &k8sHeaderExactType,
											Name:  gwv1beta1.HTTPHeaderName("exact-match-header"),
											Value: "testv1",
										},
										{
											Type:  &k8sHeaderRegexType,
											Name:  gwv1beta1.HTTPHeaderName("case-sensitive-prefix-match-header"),
											Value: "^foo",
										},
										{
											Type:  &k8sHeaderRegexType,
											Name:  gwv1beta1.HTTPHeaderName("case-insensitive-prefix-match-header"),
											Value: "(?i)^f",
										},
										{
											Type:  &k8sHeaderRegexType,
											Name:  gwv1beta1.HTTPHeaderName("case-insensitive-contains-match-header"),
											Value: "(?i)bAz",
										},
										{
											Type:  &k8sHeaderRegexType,
											Name:  gwv1beta1.HTTPHeaderName("case-insensitive-exact-match-header"),
											Value: "(?i)^baR$",
										},
									},
								},
							},
							BackendRefs: []gwv1beta1.HTTPBackendRef{
								{
									BackendRef: backendRef1,
								},
							},
						},
					},
				},
			}),
			expectedSpec: []model.RuleSpec{
				{
					StackListenerId: "listener-id",
					MatchedHeaders: []vpclattice.HeaderMatch{
						{
							Name: aws.String("exact-match-header"),
							Match: &vpclattice.HeaderMatchType{
								Exact: aws.String("testv1"),
							},
							CaseSensitive: aws.Bool(false),
						},

						{
							Name: aws.String("case-sensitive-prefix-match-header"),
							Match: &vpclattice.HeaderMatchType{
								Prefix: aws.String("foo"),
							},
							CaseSensitive: aws.Bool(true),
						},
						{
							Name: aws.String("case-insensitive-prefix-match-header"),
							Match: &vpclattice.HeaderMatchType{
								Prefix: aws.String("f"),
							},
							CaseSensitive: aws.Bool(false),
						},
						{
							Name: aws.String("case-insensitive-contains-match-header"),
							Match: &vpclattice.HeaderMatchType{
								Contains: aws.String("bAz"),
							},
							CaseSensitive: aws.Bool(false),
						},
						{
							Name: aws.String("case-insensitive-exact-match-header"),
							Match: &vpclattice.HeaderMatchType{
								Exact: aws.String("baR"),
							},
							CaseSensitive: aws.Bool(false),
						},
					},
					Action: model.RuleAction{
						TargetGroups: []*model.RuleTargetGroup{
							{
								StackTargetGroupId: "tg-0",
								Weight:             int64(*backendRef1.Weight),
							},
						},
					},
				},
			},
		},
		{
			name:         " negative 6 header match (max headers is 5)",
			wantErrIsNil: false,
			route: core.NewHTTPRoute(gwv1beta1.HTTPRoute{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "service1",
					Namespace: "default",
				},
				Spec: gwv1beta1.HTTPRouteSpec{
					CommonRouteSpec: gwv1beta1.CommonRouteSpec{
						ParentRefs: []gwv1beta1.ParentReference{
							{
								Name:        "gw1",
								SectionName: &httpSectionName,
							},
						},
					},
					Rules: []gwv1beta1.HTTPRouteRule{
						{
							Matches: []gwv1beta1.HTTPRouteMatch{
								{

									Path: &gwv1beta1.HTTPPathMatch{
										Type:  &k8sPathMatchExactType,
										Value: &path1,
									},
									Headers: []gwv1beta1.HTTPHeaderMatch{
										{
											Type:  &k8sHeaderExactType,
											Name:  gwv1beta1.HTTPHeaderName(hdr1),
											Value: hdr1Value,
										},
										{
											Type:  &k8sHeaderExactType,
											Name:  gwv1beta1.HTTPHeaderName(hdr2),
											Value: hdr2Value,
										},
										{
											Type:  &k8sHeaderExactType,
											Name:  gwv1beta1.HTTPHeaderName(hdr1),
											Value: hdr1Value,
										},
										{
											Type:  &k8sHeaderExactType,
											Name:  gwv1beta1.HTTPHeaderName(hdr2),
											Value: hdr2Value,
										},
										{
											Type:  &k8sHeaderExactType,
											Name:  gwv1beta1.HTTPHeaderName(hdr1),
											Value: hdr1Value,
										},
										{
											Type:  &k8sHeaderExactType,
											Name:  gwv1beta1.HTTPHeaderName(hdr2),
											Value: hdr2Value,
										},
									},
								},
							},
							BackendRefs: []gwv1beta1.HTTPBackendRef{
								{
									BackendRef: backendRef1,
								},
							},
						},
					},
				},
			}),
		},
		{
			name:         "Negative, multiple methods",
			wantErrIsNil: false,
			route: core.NewHTTPRoute(gwv1beta1.HTTPRoute{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "service1",
					Namespace: "default",
				},
				Spec: gwv1beta1.HTTPRouteSpec{
					CommonRouteSpec: gwv1beta1.CommonRouteSpec{
						ParentRefs: []gwv1beta1.ParentReference{
							{
								Name:        "gw1",
								SectionName: &httpSectionName,
							},
						},
					},
					Rules: []gwv1beta1.HTTPRouteRule{
						{
							Matches: []gwv1beta1.HTTPRouteMatch{
								{

									Path: &gwv1beta1.HTTPPathMatch{
										Type:  &k8sPathMatchExactType,
										Value: &path1,
									},
								},
								{

									Path: &gwv1beta1.HTTPPathMatch{
										Type:  &k8sPathMatchExactType,
										Value: &path1,
									},
								},
							},
							BackendRefs: []gwv1beta1.HTTPBackendRef{
								{
									BackendRef: backendRef1,
								},
							},
						},
					},
				},
			}),
		},
		{
			name:         "GRPC match on service and method",
			wantErrIsNil: true,
			route: core.NewGRPCRoute(gwv1alpha2.GRPCRoute{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "service1",
					Namespace: "default",
				},
				Spec: gwv1alpha2.GRPCRouteSpec{
					CommonRouteSpec: gwv1beta1.CommonRouteSpec{
						ParentRefs: []gwv1beta1.ParentReference{
							{
								Name:        "gw1",
								SectionName: &httpSectionName,
							},
						},
					},
					Rules: []gwv1alpha2.GRPCRouteRule{
						{
							Matches: []gwv1alpha2.GRPCRouteMatch{
								{
									Method: &gwv1alpha2.GRPCMethodMatch{
										Type:    &k8sMethodMatchExactType,
										Service: pointer.String("service"),
										Method:  pointer.String("method"),
									},
								},
							},
							BackendRefs: []gwv1alpha2.GRPCBackendRef{
								{
									BackendRef: backendRef1,
								},
							},
						},
					},
				},
			}),
			expectedSpec: []model.RuleSpec{
				{
					StackListenerId: "listener-id",
					PathMatchExact:  true,
					PathMatchValue:  "/service/method",
					Method:          "POST",
					Action: model.RuleAction{
						TargetGroups: []*model.RuleTargetGroup{
							{
								StackTargetGroupId: "tg-0",
								Weight:             int64(*backendRef1.Weight),
							},
						},
					},
				},
			},
		},
		{
			name:         "GRPC match on service",
			wantErrIsNil: true,
			route: core.NewGRPCRoute(gwv1alpha2.GRPCRoute{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "service1",
					Namespace: "default",
				},
				Spec: gwv1alpha2.GRPCRouteSpec{
					CommonRouteSpec: gwv1beta1.CommonRouteSpec{
						ParentRefs: []gwv1beta1.ParentReference{
							{
								Name:        "gw1",
								SectionName: &httpSectionName,
							},
						},
					},
					Rules: []gwv1alpha2.GRPCRouteRule{
						{
							Matches: []gwv1alpha2.GRPCRouteMatch{
								{
									Method: &gwv1alpha2.GRPCMethodMatch{
										Type:    &k8sMethodMatchExactType,
										Service: pointer.String("service"),
									},
								},
							},
							BackendRefs: []gwv1alpha2.GRPCBackendRef{
								{
									BackendRef: backendRef1,
								},
							},
						},
					},
				},
			}),
			expectedSpec: []model.RuleSpec{
				{
					StackListenerId: "listener-id",
					PathMatchPrefix: true,
					PathMatchValue:  "/service/",
					Method:          "POST",
					Action: model.RuleAction{
						TargetGroups: []*model.RuleTargetGroup{
							{
								StackTargetGroupId: "tg-0",
								Weight:             int64(*backendRef1.Weight),
							},
						},
					},
				},
			},
		},
		{
			name:         "GRPC match on all",
			wantErrIsNil: true,
			route: core.NewGRPCRoute(gwv1alpha2.GRPCRoute{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "service1",
					Namespace: "default",
				},
				Spec: gwv1alpha2.GRPCRouteSpec{
					CommonRouteSpec: gwv1beta1.CommonRouteSpec{
						ParentRefs: []gwv1beta1.ParentReference{
							{
								Name:        "gw1",
								SectionName: &httpSectionName,
							},
						},
					},
					Rules: []gwv1alpha2.GRPCRouteRule{
						{
							Matches: []gwv1alpha2.GRPCRouteMatch{
								{
									Method: &gwv1alpha2.GRPCMethodMatch{
										Type: &k8sMethodMatchExactType,
									},
								},
							},

							BackendRefs: []gwv1alpha2.GRPCBackendRef{
								{
									BackendRef: backendRef1,
								},
							},
						},
					},
				},
			}),
			expectedSpec: []model.RuleSpec{
				{
					StackListenerId: "listener-id",
					PathMatchPrefix: true,
					PathMatchValue:  "/",
					Method:          "POST",
					Action: model.RuleAction{
						TargetGroups: []*model.RuleTargetGroup{
							{
								StackTargetGroupId: "tg-0",
								Weight:             int64(*backendRef1.Weight),
							},
						},
					},
				},
			},
		},
		{
			name:         "GRPC match with 5 headers",
			wantErrIsNil: true,
			route: core.NewGRPCRoute(gwv1alpha2.GRPCRoute{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "service1",
					Namespace: "default",
				},
				Spec: gwv1alpha2.GRPCRouteSpec{
					CommonRouteSpec: gwv1beta1.CommonRouteSpec{
						ParentRefs: []gwv1beta1.ParentReference{
							{
								Name:        "gw1",
								SectionName: &httpSectionName,
							},
						},
					},
					Rules: []gwv1alpha2.GRPCRouteRule{
						{
							Matches: []gwv1alpha2.GRPCRouteMatch{
								{
									Method: &gwv1alpha2.GRPCMethodMatch{
										Type:    &k8sMethodMatchExactType,
										Service: pointer.String("service"),
									},
									Headers: []gwv1alpha2.GRPCHeaderMatch{
										{
											Name:  "foo1",
											Value: "bar1",
											Type:  &k8sHeaderExactType,
										},
										{
											Name:  "foo2",
											Value: "bar2",
											Type:  &k8sHeaderExactType,
										},
										{
											Name:  "foo3",
											Value: "bar3",
											Type:  &k8sHeaderExactType,
										},
										{
											Name:  "foo4",
											Value: "^bar4",
											Type:  &k8sHeaderRegexType,
										},
										{
											Name:  "foo5",
											Value: "bar5",
											Type:  &k8sHeaderExactType,
										},
									},
								},
							},
							BackendRefs: []gwv1alpha2.GRPCBackendRef{
								{
									BackendRef: backendRef1,
								},
							},
						},
					},
				},
			}),
			expectedSpec: []model.RuleSpec{
				{
					StackListenerId: "listener-id",
					PathMatchPrefix: true,
					PathMatchValue:  "/service/",
					Method:          "POST",
					Action: model.RuleAction{
						TargetGroups: []*model.RuleTargetGroup{
							{
								StackTargetGroupId: "tg-0",
								Weight:             int64(*backendRef1.Weight),
							},
						},
					},
					MatchedHeaders: []vpclattice.HeaderMatch{
						{
							Name: aws.String("foo1"),
							Match: &vpclattice.HeaderMatchType{
								Exact: aws.String("bar1"),
							},
							CaseSensitive: aws.Bool(false),
						},
						{
							Name: aws.String("foo2"),
							Match: &vpclattice.HeaderMatchType{
								Exact: aws.String("bar2"),
							},
							CaseSensitive: aws.Bool(false),
						},
						{
							Name: aws.String("foo3"),
							Match: &vpclattice.HeaderMatchType{
								Exact: aws.String("bar3"),
							},
							CaseSensitive: aws.Bool(false),
						},
						{
							Name: aws.String("foo4"),
							Match: &vpclattice.HeaderMatchType{
								Exact: aws.String("bar4"),
							},
							CaseSensitive: aws.Bool(false),
						},
						{
							Name: aws.String("foo5"),
							Match: &vpclattice.HeaderMatchType{
								Exact: aws.String("bar5"),
							},
							CaseSensitive: aws.Bool(false),
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := gomock.NewController(t)
			defer c.Finish()
			ctx := context.TODO()

			k8sSchema := runtime.NewScheme()
			k8sSchema.AddKnownTypes(mcsv1alpha1.SchemeGroupVersion, &mcsv1alpha1.ServiceImport{})
			clientgoscheme.AddToScheme(k8sSchema)
			k8sClient := testclient.NewFakeClientWithScheme(k8sSchema)

			svc := corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      string(backendRef1.Name),
					Namespace: "default",
				},
				Status: corev1.ServiceStatus{},
			}
			assert.NoError(t, k8sClient.Create(ctx, svc.DeepCopy()))
			stack := core.NewDefaultStack(core.StackID(k8s.NamespacedName(tt.route.K8sObject())))

			task := &latticeServiceModelBuildTask{
				log:         gwlog.FallbackLogger,
				route:       tt.route,
				stack:       stack,
				client:      k8sClient,
				brTgBuilder: &dummyTgBuilder{},
			}

			err := task.buildRules(ctx, "listener-id")
			if tt.wantErrIsNil {
				assert.NoError(t, err)
			} else {
				assert.NotNil(t, err)
				return
			}

			var resRules []*model.Rule
			stack.ListResources(&resRules)

			validateEqual(t, tt.expectedSpec, resRules)
		})
	}
}

func validateEqual(t *testing.T, expectedRules []model.RuleSpec, actualRules []*model.Rule) {
	assert.Equal(t, len(expectedRules), len(actualRules))
	assert.Equal(t, len(expectedRules), len(actualRules))

	for i, expectedSpec := range expectedRules {
		actualRule := actualRules[i]

		assert.Equal(t, expectedSpec.StackListenerId, actualRule.Spec.StackListenerId)
		assert.Equal(t, expectedSpec.PathMatchValue, actualRule.Spec.PathMatchValue)
		assert.Equal(t, expectedSpec.PathMatchPrefix, actualRule.Spec.PathMatchPrefix)
		assert.Equal(t, expectedSpec.PathMatchExact, actualRule.Spec.PathMatchExact)
		assert.Equal(t, expectedSpec.Method, actualRule.Spec.Method)

		// priority is not determined by model building, but in synthesis, so we don't
		// validate priority here

		assert.Equal(t, expectedSpec.MatchedHeaders, actualRule.Spec.MatchedHeaders)

		assert.Equal(t, len(expectedSpec.Action.TargetGroups), len(actualRule.Spec.Action.TargetGroups))
		for j, etg := range expectedSpec.Action.TargetGroups {
			atg := actualRule.Spec.Action.TargetGroups[j]

			assert.Equal(t, etg.Weight, atg.Weight)
			assert.Equal(t, etg.StackTargetGroupId, atg.StackTargetGroupId)
			assert.Equal(t, etg.SvcImportTG, etg.SvcImportTG)
		}
	}
}

func Test_toControllerSupportedHeaderMatch(t *testing.T) {
	tests := []struct {
		name          string
		regex         string
		expectedType  LatticeHeaderMatchType
		caseSensitive bool
		value         string
		wantErr       bool
	}{
		// Valid cases
		{"ValidCaseSensitivePrefix", "^foo", LatticeHeaderMatchTypePrefix, true, "foo", false},
		{"ValidCaseSensitiveContains", "foo", LatticeHeaderMatchTypeContains, true, "foo", false},
		{"ValidCaseSensitiveExact", "^baz$", LatticeHeaderMatchTypeExact, true, "baz", false},
		{"ValidCaseInsensitivePrefix", "(?i)^foo", LatticeHeaderMatchTypePrefix, false, "foo", false},
		{"ValidCaseInsensitiveContains", "(?i)bAr", LatticeHeaderMatchTypeContains, false, "bAr", false},
		{"ValidCaseInsensitiveExact", "(?i)^baz$", LatticeHeaderMatchTypeExact, false, "baz", false},
		{"ValidAlphanumericLiteral", "123fooABC", LatticeHeaderMatchTypeContains, true, "123fooABC", false},
		{"ValidNumericLiteral", "123456", LatticeHeaderMatchTypeContains, true, "123456", false},
		{"Valid CaseInsensitive with dash", "(?i)my-header-value", LatticeHeaderMatchTypeContains, false, "my-header-value", false},
		{"Valid prefix Case sensitive with underscore", "^my_header_value", LatticeHeaderMatchTypePrefix, true, "my_header_value", false},
		//{"Valid prefix Case sensitive with underscore", "^my_header_value", LatticeHeaderMatchTypePrefix, true, "my_header_value", false},

		// Invalid cases - unsupported syntax for current regex subset
		{"InvalidRegexWithGroup", "(?i)foo(bar)", "", false, "", true},
		{"InvalidRegexWithOr", "foo|bar", "", false, "", true},
		{"InvalidRegexWithEndAnchor", "foo$", "", false, "", true},
		{"InvalidRegexWithStartAndGroup", "^foo(bar)", "", false, "", true},
		{"InvalidRegexWithCharacterSet", "[a-z]+", "", false, "", true},
		{"InvalidRegexWithQuantifier", "foo*", "", false, "", true},
		{"InvalidRegexWithQuantifierPlus", "foo+", "", false, "", true},
		{"InvalidRegexWithDigits", "\\d{3}", "", false, "", true},
		{"InvalidRegexWithWordBoundary", "\\bfoo", "", false, "", true},
		{"InvalidRegexWithEscapeChars", "foo\\nbar", "", false, "", true},

		// Invalid cases - literals with unsupported characters or spaces
		{"InvalidLiteralWithSpace", "just a string", "", false, "", true},
		{"InvalidLiteralWithSpaceInPrefix", "^just a string", "", false, "", true},
		{"InvalidLiteralWithSpaceInExact", "^just a string$", "", false, "", true},

		// Edge cases
		{"EmptyString", "", LatticeHeaderMatchTypePrefix, false, "", false},
		{"OnlyCaret", "^", LatticeHeaderMatchTypePrefix, false, "", false},
		{"OnlyDollar", "$", "", false, "", true},
		{"CaretAndDollar", "^$", LatticeHeaderMatchTypeExact, false, "", false},
		{"CaretAndDollarCaseInsensitive", "(?i)^$", LatticeHeaderMatchTypeExact, false, "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotType, gotCaseSensitive, gotValue, err := toLatticeHeaderMatch(tt.regex)
			if (err != nil) != tt.wantErr {
				t.Errorf("toLatticeHeaderMatch() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if gotType != tt.expectedType {
					t.Errorf("toLatticeHeaderMatch() gotType = %v, want %v", gotType, tt.expectedType)
				}
				if gotCaseSensitive != tt.caseSensitive {
					t.Errorf("toLatticeHeaderMatch() gotCaseSensitive = %v, want %v", gotCaseSensitive, tt.caseSensitive)
				}
				if gotValue != tt.value {
					t.Errorf("toLatticeHeaderMatch() gotValue = %v, want %v", gotValue, tt.value)
				}
			}
		})
	}
}
