package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
	//"k8s.io/utils/ptr"
	gwv1 "sigs.k8s.io/gateway-api/apis/v1"
	gwv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
)

func TestTLSRouteSpec_Equals(t *testing.T) {

	tests := []struct {
		routeSpec1  *TLSRouteSpec
		routeSpec2  RouteSpec
		expectEqual bool
		description string
	}{
		{
			description: "Empty instance are equal",
			routeSpec1:  &TLSRouteSpec{},
			routeSpec2:  &TLSRouteSpec{},
			expectEqual: true,
		},
		{
			description: "Instance populated with the same values are equal",
			routeSpec1: &TLSRouteSpec{
				s: gwv1alpha2.TLSRouteSpec{
					CommonRouteSpec: gwv1.CommonRouteSpec{
						ParentRefs: []gwv1.ParentReference{
							{},
						},
					},
					Hostnames: []gwv1alpha2.Hostname{"example.com"},
					Rules: []gwv1alpha2.TLSRouteRule{
						{},
					},
				},
			},
			routeSpec2: &TLSRouteSpec{
				s: gwv1alpha2.TLSRouteSpec{
					CommonRouteSpec: gwv1.CommonRouteSpec{
						ParentRefs: []gwv1.ParentReference{
							{},
						},
					},
					Hostnames: []gwv1alpha2.Hostname{"example.com"},
					Rules: []gwv1alpha2.TLSRouteRule{
						{},
					},
				},
			},
			expectEqual: true,
		},
		{
			description: "Instances of different types are not equal",
			routeSpec1:  &TLSRouteSpec{},
			routeSpec2:  &HTTPRouteSpec{},
			expectEqual: false,
		},
		{
			description: "Instance with different ParentRefs are not equal",
			routeSpec1: &TLSRouteSpec{
				s: gwv1alpha2.TLSRouteSpec{
					CommonRouteSpec: gwv1alpha2.CommonRouteSpec{
						ParentRefs: []gwv1alpha2.ParentReference{{Name: "parent1"}},
					},
				},
			},
			routeSpec2: &TLSRouteSpec{
				s: gwv1alpha2.TLSRouteSpec{
					CommonRouteSpec: gwv1alpha2.CommonRouteSpec{
						ParentRefs: []gwv1alpha2.ParentReference{{Name: "parent2"}},
					},
				},
			},
			expectEqual: false,
		},
		{
			description: "Instance with different Hostnames are not equal",
			routeSpec1: &TLSRouteSpec{
				s: gwv1alpha2.TLSRouteSpec{
					Hostnames: []gwv1alpha2.Hostname{"example1.com"},
				},
			},
			routeSpec2: &TLSRouteSpec{
				s: gwv1alpha2.TLSRouteSpec{
					Hostnames: []gwv1alpha2.Hostname{"example2.com"},
				},
			},
			expectEqual: false,
		},
		{
			routeSpec1:  &TLSRouteSpec{},
			routeSpec2:  nil,
			expectEqual: false,
			description: "Non-nil instances are not equal to nil",
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			assert.Equal(t, test.expectEqual, test.routeSpec1.Equals(test.routeSpec2), test.description)
		})
	}
}
