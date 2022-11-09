package gateway

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	v1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"

	"github.com/aws/aws-application-networking-k8s/pkg/config"
	"github.com/aws/aws-application-networking-k8s/pkg/k8s"
	"github.com/aws/aws-application-networking-k8s/pkg/model/core"
	latticemodel "github.com/aws/aws-application-networking-k8s/pkg/model/lattice"
)

const (
	ResourceIDServiceNetwork = "ServiceNetwork"
)

// ModelBuilder builds the model stack for the mesh resource.
type ServiceNetworkModelBuilder interface {
	// Build model stack for service
	Build(ctx context.Context, gw *v1alpha2.Gateway) (core.Stack, *latticemodel.ServiceNetwork, error)
}

type serviceNetworkModelBuilder struct {
	defaultTags map[string]string
}

func NewServiceNetworkModelBuilder() *serviceNetworkModelBuilder {
	return &serviceNetworkModelBuilder{}
}
func (b *serviceNetworkModelBuilder) Build(ctx context.Context, gw *v1alpha2.Gateway) (core.Stack, *latticemodel.ServiceNetwork, error) {
	stack := core.NewDefaultStack(core.StackID(k8s.NamespacedName(gw)))

	task := &serviceNetworkModelBuildTask{
		gateway: gw,
		stack:   stack,
	}

	if err := task.run(ctx); err != nil {
		return nil, nil, corev1.ErrIntOverflowGenerated
	}

	return task.stack, task.mesh, nil
}

func (t *serviceNetworkModelBuildTask) run(ctx context.Context) error {

	err := t.buildModel(ctx)
	return err
}

func (t *serviceNetworkModelBuildTask) buildModel(ctx context.Context) error {
	err := t.buildServiceNetwork(ctx)
	if err != nil {
		return err
	}

	return nil
}

func (t *serviceNetworkModelBuildTask) buildServiceNetwork(ctx context.Context) error {
	spec := latticemodel.ServiceNetworkSpec{
		Name:    t.gateway.Name,
		Account: config.AccountID,
	}

	if !t.gateway.DeletionTimestamp.IsZero() {
		spec.IsDeleted = true
	} else {
		spec.IsDeleted = false
	}

	t.mesh = latticemodel.NewServiceNetwork(t.stack, ResourceIDServiceNetwork, spec)

	return nil
}

type serviceNetworkModelBuildTask struct {
	gateway *v1alpha2.Gateway

	mesh *latticemodel.ServiceNetwork

	stack core.Stack
}