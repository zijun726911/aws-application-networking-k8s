package lattice

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/vpclattice"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"github.com/aws/aws-application-networking-k8s/pkg/config"
	"github.com/aws/aws-application-networking-k8s/pkg/utils/gwlog"

	mocks_aws "github.com/aws/aws-application-networking-k8s/pkg/aws"
	mocks "github.com/aws/aws-application-networking-k8s/pkg/aws/services"
	latticemodel "github.com/aws/aws-application-networking-k8s/pkg/model/lattice"
)

// ServiceNetwork does not exist before,happy case.
func Test_CreateOrUpdateServiceNetwork_MeshNotExist_NoNeedToAssociate(t *testing.T) {
	meshCreateInput := latticemodel.ServiceNetwork{
		Spec: latticemodel.ServiceNetworkSpec{
			Name:           "test",
			Account:        "123456789",
			AssociateToVPC: false,
		},
		Status: &latticemodel.ServiceNetworkStatus{ServiceNetworkARN: "", ServiceNetworkID: ""},
	}
	arn := "12345678912345678912"
	id := "12345678912345678912"
	name := "test"
	meshCreateOutput := &vpclattice.CreateServiceNetworkOutput{
		Arn:  &arn,
		Id:   &id,
		Name: &name,
	}
	createServiceNetworkInput := &vpclattice.CreateServiceNetworkInput{
		Name: &name,
		Tags: make(map[string]*string),
	}
	createServiceNetworkInput.Tags[latticemodel.K8SServiceNetworkOwnedByVPC] = &config.VpcID

	c := gomock.NewController(t)
	defer c.Finish()
	ctx := context.TODO()
	mockCloud := mocks_aws.NewMockCloud(c)
	mockLattice := mocks.NewMockLattice(c)
	mockLattice.EXPECT().CreateServiceNetworkWithContext(ctx, createServiceNetworkInput).Return(meshCreateOutput, nil)
	mockCloud.EXPECT().Lattice().Return(mockLattice).AnyTimes()
	mockLattice.EXPECT().FindServiceNetwork(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, &mocks.NotFoundError{}).Times(1)
	meshManager := NewDefaultServiceNetworkManager(gwlog.FallbackLogger, mockCloud)
	resp, err := meshManager.CreateOrUpdate(ctx, &meshCreateInput)

	assert.Nil(t, err)
	assert.Equal(t, resp.ServiceNetworkARN, arn)
	assert.Equal(t, resp.ServiceNetworkID, id)
}

func Test_CreateOrUpdateServiceNetwork_MeshNotExist_NeedToAssociate(t *testing.T) {
	meshCreateInput := latticemodel.ServiceNetwork{
		Spec: latticemodel.ServiceNetworkSpec{
			Name:           "test",
			Account:        "123456789",
			AssociateToVPC: true,
		},
		Status: &latticemodel.ServiceNetworkStatus{ServiceNetworkARN: "", ServiceNetworkID: ""},
	}
	arn := "12345678912345678912"
	id := "12345678912345678912"
	name := "test"
	meshCreateOutput := &vpclattice.CreateServiceNetworkOutput{
		Arn:  &arn,
		Id:   &id,
		Name: &name,
	}

	createServiceNetworkInput := &vpclattice.CreateServiceNetworkInput{
		Name: &name,
		Tags: make(map[string]*string),
	}
	createServiceNetworkInput.Tags[latticemodel.K8SServiceNetworkOwnedByVPC] = &config.VpcID

	c := gomock.NewController(t)
	defer c.Finish()
	ctx := context.TODO()
	mockCloud := mocks_aws.NewMockCloud(c)
	mockLattice := mocks.NewMockLattice(c)

	mockLattice.EXPECT().CreateServiceNetworkWithContext(ctx, createServiceNetworkInput).Return(meshCreateOutput, nil)
	meshId := "12345678912345678912"
	createServiceNetworkVpcAssociationInput := &vpclattice.CreateServiceNetworkVpcAssociationInput{
		ServiceNetworkIdentifier: &meshId,
		VpcIdentifier:            &config.VpcID,
	}
	associationStatus := vpclattice.ServiceNetworkVpcAssociationStatusUpdateInProgress
	createServiceNetworkVPCAssociationOutput := &vpclattice.CreateServiceNetworkVpcAssociationOutput{
		Status: &associationStatus,
	}
	mockLattice.EXPECT().
		CreateServiceNetworkVpcAssociationWithContext(ctx, createServiceNetworkVpcAssociationInput).
		Return(createServiceNetworkVPCAssociationOutput, nil)

	mockLattice.EXPECT().
		FindServiceNetwork(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)

	mockCloud.EXPECT().Lattice().Return(mockLattice).AnyTimes()

	meshManager := NewDefaultServiceNetworkManager(gwlog.FallbackLogger, mockCloud)
	resp, err := meshManager.CreateOrUpdate(ctx, &meshCreateInput)

	assert.Nil(t, err)
	assert.Equal(t, resp.ServiceNetworkARN, arn)
	assert.Equal(t, resp.ServiceNetworkID, id)
}

// List and find mesh does not work.
func Test_CreateOrUpdateServiceNetwork_ListFailed(t *testing.T) {
	meshCreateInput := latticemodel.ServiceNetwork{
		Spec: latticemodel.ServiceNetworkSpec{
			Name:    "test",
			Account: "123456789",
		},
		Status: &latticemodel.ServiceNetworkStatus{ServiceNetworkARN: "", ServiceNetworkID: ""},
	}

	c := gomock.NewController(t)
	defer c.Finish()
	ctx := context.TODO()
	mockLattice := mocks.NewMockLattice(c)
	mockCloud := mocks_aws.NewMockCloud(c)

	mockLattice.EXPECT().FindServiceNetwork(ctx, gomock.Any(), gomock.Any()).Return(nil, errors.New("ERROR"))
	mockCloud.EXPECT().Lattice().Return(mockLattice).AnyTimes()

	meshManager := NewDefaultServiceNetworkManager(gwlog.FallbackLogger, mockCloud)
	resp, err := meshManager.CreateOrUpdate(ctx, &meshCreateInput)

	assert.NotNil(t, err)
	assert.Equal(t, resp.ServiceNetworkARN, "")
	assert.Equal(t, resp.ServiceNetworkID, "")
}

// ServiceNetwork already exists, association is in ServiceNetworkVpcAssociationStatusCreateInProgress.

func Test_CreateOrUpdateServiceNetwork_MeshAlreadyExist_ServiceNetworkVpcAssociationStatusCreateInProgress(t *testing.T) {
	meshCreateInput := latticemodel.ServiceNetwork{
		Spec: latticemodel.ServiceNetworkSpec{
			Name:           "test",
			Account:        "123456789",
			AssociateToVPC: true,
		},
		Status: &latticemodel.ServiceNetworkStatus{ServiceNetworkARN: "", ServiceNetworkID: ""},
	}
	meshId := "12345678912345678912"
	meshArn := "12345678912345678912"
	name := "test"
	vpcId := config.VpcID
	item := vpclattice.ServiceNetworkSummary{
		Arn:  &meshArn,
		Id:   &meshId,
		Name: &name,
	}

	status := vpclattice.ServiceNetworkVpcAssociationStatusCreateInProgress
	items := vpclattice.ServiceNetworkVpcAssociationSummary{
		ServiceNetworkArn:  &meshArn,
		ServiceNetworkId:   &meshId,
		ServiceNetworkName: &meshId,
		Status:             &status,
		VpcId:              &vpcId,
	}
	statusServiceNetworkVPCOutput := []*vpclattice.ServiceNetworkVpcAssociationSummary{&items}

	c := gomock.NewController(t)
	defer c.Finish()
	ctx := context.TODO()
	mockLattice := mocks.NewMockLattice(c)
	mockCloud := mocks_aws.NewMockCloud(c)
	mockLattice.EXPECT().ListServiceNetworkVpcAssociationsAsList(ctx, gomock.Any()).Return(statusServiceNetworkVPCOutput, nil)
	mockLattice.EXPECT().FindServiceNetwork(ctx, gomock.Any(), gomock.Any()).Return(
		&mocks.ServiceNetworkInfo{
			SvcNetwork: item,
			Tags:       nil,
		}, nil)
	mockCloud.EXPECT().Lattice().Return(mockLattice).AnyTimes()

	meshManager := NewDefaultServiceNetworkManager(gwlog.FallbackLogger, mockCloud)
	resp, err := meshManager.CreateOrUpdate(ctx, &meshCreateInput)

	assert.NotNil(t, err)
	assert.Equal(t, err, errors.New(LATTICE_RETRY))
	assert.Equal(t, resp.ServiceNetworkARN, "")
	assert.Equal(t, resp.ServiceNetworkID, "")
}

// ServiceNetwork already exists, association is in ServiceNetworkVpcAssociationStatusDeleteInProgress.

func Test_CreateOrUpdateServiceNetwork_MeshAlreadyExist_ServiceNetworkVpcAssociationStatusDeleteInProgress(t *testing.T) {
	meshCreateInput := latticemodel.ServiceNetwork{
		Spec: latticemodel.ServiceNetworkSpec{
			Name:    "test",
			Account: "123456789",
		},
		Status: &latticemodel.ServiceNetworkStatus{ServiceNetworkARN: "", ServiceNetworkID: ""},
	}
	meshId := "12345678912345678912"
	meshArn := "12345678912345678912"
	name := "test"
	vpcId := config.VpcID
	item := vpclattice.ServiceNetworkSummary{
		Arn:  &meshArn,
		Id:   &meshId,
		Name: &name,
	}

	status := vpclattice.ServiceNetworkVpcAssociationStatusDeleteInProgress
	items := vpclattice.ServiceNetworkVpcAssociationSummary{
		ServiceNetworkArn:  &meshArn,
		ServiceNetworkId:   &meshId,
		ServiceNetworkName: &meshId,
		Status:             &status,
		VpcId:              &vpcId,
	}
	statusServiceNetworkVPCOutput := []*vpclattice.ServiceNetworkVpcAssociationSummary{&items}

	c := gomock.NewController(t)
	defer c.Finish()
	ctx := context.TODO()
	mockLattice := mocks.NewMockLattice(c)
	mockCloud := mocks_aws.NewMockCloud(c)
	mockLattice.EXPECT().ListServiceNetworkVpcAssociationsAsList(ctx, gomock.Any()).Return(statusServiceNetworkVPCOutput, nil)
	mockLattice.EXPECT().FindServiceNetwork(ctx, gomock.Any(), gomock.Any()).Return(
		&mocks.ServiceNetworkInfo{
			SvcNetwork: item,
			Tags:       nil,
		}, nil)
	mockCloud.EXPECT().Lattice().Return(mockLattice).AnyTimes()

	meshManager := NewDefaultServiceNetworkManager(gwlog.FallbackLogger, mockCloud)
	resp, err := meshManager.CreateOrUpdate(ctx, &meshCreateInput)

	assert.NotNil(t, err)
	assert.Equal(t, err, errors.New(LATTICE_RETRY))
	assert.Equal(t, resp.ServiceNetworkARN, "")
	assert.Equal(t, resp.ServiceNetworkID, "")
}

// ServiceNetwork already exists, association is ServiceNetworkVpcAssociationStatusActive.
func Test_CreateOrUpdateServiceNetwork_MeshAlreadyExist_ServiceNetworkVpcAssociationStatusActive(t *testing.T) {
	meshCreateInput := latticemodel.ServiceNetwork{
		Spec: latticemodel.ServiceNetworkSpec{
			Name:           "test",
			Account:        "123456789",
			AssociateToVPC: true,
		},
		Status: &latticemodel.ServiceNetworkStatus{ServiceNetworkARN: "", ServiceNetworkID: ""},
	}
	meshId := "12345678912345678912"
	meshArn := "12345678912345678912"
	name := "test"
	vpcId := config.VpcID
	item := vpclattice.ServiceNetworkSummary{
		Arn:  &meshArn,
		Id:   &meshId,
		Name: &name,
	}

	status := vpclattice.ServiceNetworkVpcAssociationStatusActive
	items := vpclattice.ServiceNetworkVpcAssociationSummary{
		ServiceNetworkArn:  &meshArn,
		ServiceNetworkId:   &meshId,
		ServiceNetworkName: &meshId,
		Status:             &status,
		VpcId:              &config.VpcID,
	}
	statusServiceNetworkVPCOutput := []*vpclattice.ServiceNetworkVpcAssociationSummary{&items}

	c := gomock.NewController(t)
	defer c.Finish()
	ctx := context.TODO()
	mockLattice := mocks.NewMockLattice(c)
	mockCloud := mocks_aws.NewMockCloud(c)
	mockLattice.EXPECT().FindServiceNetwork(ctx, gomock.Any(), gomock.Any()).Return(
		&mocks.ServiceNetworkInfo{
			SvcNetwork: item,
			Tags:       nil,
		}, nil)
	mockLattice.EXPECT().GetServiceNetworkVpcAssociationWithContext(ctx, gomock.Any()).Return(&vpclattice.GetServiceNetworkVpcAssociationOutput{
		ServiceNetworkArn:  &meshArn,
		ServiceNetworkId:   &meshId,
		ServiceNetworkName: &name,
		Status:             aws.String(vpclattice.ServiceNetworkVpcAssociationStatusActive),
		VpcId:              &vpcId,
	}, nil)
	mockLattice.EXPECT().ListServiceNetworkVpcAssociationsAsList(ctx, gomock.Any()).Return(statusServiceNetworkVPCOutput, nil)
	mockLattice.EXPECT().UpdateServiceNetworkVpcAssociationWithContext(ctx, gomock.Any()).Times(0)
	mockCloud.EXPECT().Lattice().Return(mockLattice).AnyTimes()

	meshManager := NewDefaultServiceNetworkManager(gwlog.FallbackLogger, mockCloud)
	resp, err := meshManager.CreateOrUpdate(ctx, &meshCreateInput)

	assert.Nil(t, err)
	assert.Equal(t, resp.ServiceNetworkARN, meshArn)
	assert.Equal(t, resp.ServiceNetworkID, meshId)
}

func Test_defaultServiceNetworkManager_CreateOrUpdate_SnExists_SnvaExists_UpdateSNVASecurityGroups(t *testing.T) {
	securityGroupIds := []*string{aws.String("sg-123456789"), aws.String("sg-987654321")}
	meshCreateInput := latticemodel.ServiceNetwork{
		Spec: latticemodel.ServiceNetworkSpec{
			Name:             "test",
			Account:          "123456789",
			AssociateToVPC:   true,
			SecurityGroupIds: securityGroupIds,
		},
	}
	meshId := "12345678912345678912"
	meshArn := "12345678912345678912"
	name := "test"
	vpcId := config.VpcID
	item := vpclattice.ServiceNetworkSummary{
		Arn:  &meshArn,
		Id:   &meshId,
		Name: &name,
	}

	status := vpclattice.ServiceNetworkVpcAssociationStatusActive
	items := vpclattice.ServiceNetworkVpcAssociationSummary{
		ServiceNetworkArn:  &meshArn,
		ServiceNetworkId:   &meshId,
		ServiceNetworkName: &meshId,
		Status:             &status,
		VpcId:              &config.VpcID,
	}
	statusServiceNetworkVPCOutput := []*vpclattice.ServiceNetworkVpcAssociationSummary{&items}

	c := gomock.NewController(t)
	defer c.Finish()
	ctx := context.TODO()
	mockLattice := mocks.NewMockLattice(c)
	mockCloud := mocks_aws.NewMockCloud(c)
	mockLattice.EXPECT().FindServiceNetwork(ctx, gomock.Any(), gomock.Any()).Return(
		&mocks.ServiceNetworkInfo{
			SvcNetwork: item,
			Tags:       nil,
		}, nil)
	mockLattice.EXPECT().GetServiceNetworkVpcAssociationWithContext(ctx, gomock.Any()).Return(&vpclattice.GetServiceNetworkVpcAssociationOutput{
		ServiceNetworkArn:  &meshArn,
		ServiceNetworkId:   &meshId,
		ServiceNetworkName: &name,
		Status:             aws.String(vpclattice.ServiceNetworkVpcAssociationStatusActive),
		VpcId:              &vpcId,
		//SecurityGroupIds:   []*string{aws.String("sg-123456789"), aws.String("sg-987654321")},
	}, nil)
	mockLattice.EXPECT().ListServiceNetworkVpcAssociationsAsList(ctx, gomock.Any()).Return(statusServiceNetworkVPCOutput, nil)
	mockLattice.EXPECT().CreateServiceNetworkServiceAssociationWithContext(ctx, gomock.Any()).MaxTimes(0)
	mockLattice.EXPECT().UpdateServiceNetworkVpcAssociationWithContext(ctx, gomock.Any()).Return(&vpclattice.UpdateServiceNetworkVpcAssociationOutput{
		Arn:              &meshArn,
		Id:               &meshId,
		SecurityGroupIds: securityGroupIds,
		Status:           aws.String(vpclattice.ServiceNetworkVpcAssociationStatusActive),
	}, nil)

	mockCloud.EXPECT().Lattice().Return(mockLattice).AnyTimes()
	meshManager := NewDefaultServiceNetworkManager(gwlog.FallbackLogger, mockCloud)
	resp, err := meshManager.CreateOrUpdate(ctx, &meshCreateInput)

	assert.Equal(t, err, nil)
	assert.Equal(t, resp.ServiceNetworkARN, meshArn)
	assert.Equal(t, resp.ServiceNetworkID, meshId)
	assert.Equal(t, resp.SnvaSecurityGroupIds, securityGroupIds)
}

func Test_defaultServiceNetworkManager_CreateOrUpdate_SnExists_SnvaExists_SecurityGroupsDoNotNeedToBeUpdated(t *testing.T) {
	securityGroupIds := []*string{aws.String("sg-123456789"), aws.String("sg-987654321")}
	desiredSn := latticemodel.ServiceNetwork{
		Spec: latticemodel.ServiceNetworkSpec{
			Name:             "test",
			Account:          "123456789",
			AssociateToVPC:   true,
			SecurityGroupIds: securityGroupIds,
		},
	}
	meshId := "12345678912345678912"
	meshArn := "12345678912345678912"
	name := "test"
	vpcId := config.VpcID
	item := vpclattice.ServiceNetworkSummary{
		Arn:  &meshArn,
		Id:   &meshId,
		Name: &name,
	}

	status := vpclattice.ServiceNetworkVpcAssociationStatusActive
	items := vpclattice.ServiceNetworkVpcAssociationSummary{
		ServiceNetworkArn:  &meshArn,
		ServiceNetworkId:   &meshId,
		ServiceNetworkName: &meshId,
		Status:             &status,
		VpcId:              &config.VpcID,
	}
	statusServiceNetworkVPCOutput := []*vpclattice.ServiceNetworkVpcAssociationSummary{&items}

	c := gomock.NewController(t)
	defer c.Finish()
	ctx := context.TODO()
	mockLattice := mocks.NewMockLattice(c)
	mockCloud := mocks_aws.NewMockCloud(c)
	mockLattice.EXPECT().FindServiceNetwork(ctx, gomock.Any(), gomock.Any()).Return(
		&mocks.ServiceNetworkInfo{
			SvcNetwork: item,
			Tags:       nil,
		}, nil)
	mockLattice.EXPECT().GetServiceNetworkVpcAssociationWithContext(ctx, gomock.Any()).Return(&vpclattice.GetServiceNetworkVpcAssociationOutput{
		ServiceNetworkArn:  &meshArn,
		ServiceNetworkId:   &meshId,
		ServiceNetworkName: &name,
		Status:             &status,
		VpcId:              &vpcId,
		SecurityGroupIds:   securityGroupIds,
	}, nil)
	mockLattice.EXPECT().ListServiceNetworkVpcAssociationsAsList(ctx, gomock.Any()).Return(statusServiceNetworkVPCOutput, nil)
	mockLattice.EXPECT().CreateServiceNetworkServiceAssociationWithContext(ctx, gomock.Any()).Times(0)
	mockLattice.EXPECT().UpdateServiceNetworkVpcAssociationWithContext(ctx, gomock.Any()).Times(0)

	mockCloud.EXPECT().Lattice().Return(mockLattice).AnyTimes()
	meshManager := NewDefaultServiceNetworkManager(gwlog.FallbackLogger, mockCloud)
	resp, err := meshManager.CreateOrUpdate(ctx, &desiredSn)

	assert.Equal(t, err, nil)
	assert.Equal(t, resp.ServiceNetworkARN, meshArn)
	assert.Equal(t, resp.ServiceNetworkID, meshId)
	assert.Equal(t, resp.SnvaSecurityGroupIds, securityGroupIds)
}
func Test_defaultServiceNetworkManager_CreateOrUpdate_SnExists_SnvaCreateInProgress_WillNotInvokeLatticeUpdateSNVA(t *testing.T) {
	securityGroupIds := []*string{aws.String("sg-123456789"), aws.String("sg-987654321")}
	desiredSn := latticemodel.ServiceNetwork{
		Spec: latticemodel.ServiceNetworkSpec{
			Name:             "test",
			Account:          "123456789",
			AssociateToVPC:   true,
			SecurityGroupIds: securityGroupIds,
		},
	}
	meshId := "12345678912345678912"
	meshArn := "12345678912345678912"
	name := "test"
	item := vpclattice.ServiceNetworkSummary{
		Arn:  &meshArn,
		Id:   &meshId,
		Name: &name,
	}

	status := vpclattice.ServiceNetworkVpcAssociationStatusCreateInProgress
	items := vpclattice.ServiceNetworkVpcAssociationSummary{
		ServiceNetworkArn:  &meshArn,
		ServiceNetworkId:   &meshId,
		ServiceNetworkName: &meshId,
		Status:             &status,
		VpcId:              &config.VpcID,
	}
	statusServiceNetworkVPCOutput := []*vpclattice.ServiceNetworkVpcAssociationSummary{&items}

	c := gomock.NewController(t)
	defer c.Finish()
	ctx := context.TODO()
	mockLattice := mocks.NewMockLattice(c)
	mockCloud := mocks_aws.NewMockCloud(c)
	mockLattice.EXPECT().FindServiceNetwork(ctx, gomock.Any(), gomock.Any()).Return(
		&mocks.ServiceNetworkInfo{
			SvcNetwork: item,
			Tags:       nil,
		}, nil)
	mockLattice.EXPECT().GetServiceNetworkVpcAssociationWithContext(ctx, gomock.Any()).Times(0)
	mockLattice.EXPECT().ListServiceNetworkVpcAssociationsAsList(ctx, gomock.Any()).Return(statusServiceNetworkVPCOutput, nil)
	mockLattice.EXPECT().CreateServiceNetworkServiceAssociationWithContext(ctx, gomock.Any()).Times(0)
	mockLattice.EXPECT().UpdateServiceNetworkVpcAssociationWithContext(ctx, gomock.Any()).Times(0)

	mockCloud.EXPECT().Lattice().Return(mockLattice).AnyTimes()
	meshManager := NewDefaultServiceNetworkManager(gwlog.FallbackLogger, mockCloud)
	resp, err := meshManager.CreateOrUpdate(ctx, &desiredSn)

	assert.Equal(t, err, errors.New(LATTICE_RETRY))
	assert.Equal(t, resp.ServiceNetworkARN, "")
	assert.Equal(t, resp.ServiceNetworkID, "")
}

func Test_defaultServiceNetworkManager_CreateOrUpdate_SnExists_SnvaExists_CannotUpdateSecurityGroupsFromNonemptyToEmpty(t *testing.T) {
	securityGroupIds := []*string{aws.String("sg-123456789"), aws.String("sg-987654321")}
	desiredSn := latticemodel.ServiceNetwork{
		Spec: latticemodel.ServiceNetworkSpec{
			Name:             "test",
			Account:          "123456789",
			AssociateToVPC:   true,
			SecurityGroupIds: []*string{},
		},
	}
	meshId := "12345678912345678912"
	meshArn := "12345678912345678912"
	name := "test"
	item := vpclattice.ServiceNetworkSummary{
		Arn:  &meshArn,
		Id:   &meshId,
		Name: &name,
	}

	status := vpclattice.ServiceNetworkVpcAssociationStatusActive
	items := vpclattice.ServiceNetworkVpcAssociationSummary{
		ServiceNetworkArn:  &meshArn,
		ServiceNetworkId:   &meshId,
		ServiceNetworkName: &meshId,
		Status:             &status,
		VpcId:              &config.VpcID,
	}
	statusServiceNetworkVPCOutput := []*vpclattice.ServiceNetworkVpcAssociationSummary{&items}

	c := gomock.NewController(t)
	defer c.Finish()
	ctx := context.TODO()
	mockLattice := mocks.NewMockLattice(c)
	mockCloud := mocks_aws.NewMockCloud(c)
	mockLattice.EXPECT().FindServiceNetwork(ctx, gomock.Any(), gomock.Any()).Return(
		&mocks.ServiceNetworkInfo{
			SvcNetwork: item,
			Tags:       nil,
		}, nil)
	mockLattice.EXPECT().GetServiceNetworkVpcAssociationWithContext(ctx, gomock.Any()).Return(&vpclattice.GetServiceNetworkVpcAssociationOutput{
		ServiceNetworkArn:  &meshArn,
		ServiceNetworkId:   &meshId,
		ServiceNetworkName: &name,
		Status:             aws.String(vpclattice.ServiceNetworkVpcAssociationStatusActive),
		VpcId:              &config.VpcID,
		SecurityGroupIds:   securityGroupIds,
	}, nil)
	mockLattice.EXPECT().ListServiceNetworkVpcAssociationsAsList(ctx, gomock.Any()).Return(statusServiceNetworkVPCOutput, nil)
	mockLattice.EXPECT().CreateServiceNetworkServiceAssociationWithContext(ctx, gomock.Any()).Times(0)
	updateSNVAError := errors.New("InvalidParameterException SecurityGroupIds cannot be empty")
	mockLattice.EXPECT().UpdateServiceNetworkVpcAssociationWithContext(ctx, gomock.Any()).Return(&vpclattice.UpdateServiceNetworkVpcAssociationOutput{}, updateSNVAError)

	mockCloud.EXPECT().Lattice().Return(mockLattice).AnyTimes()
	meshManager := NewDefaultServiceNetworkManager(gwlog.FallbackLogger, mockCloud)
	resp, err := meshManager.CreateOrUpdate(ctx, &desiredSn)

	assert.Equal(t, err, updateSNVAError)
	assert.Equal(t, resp.ServiceNetworkARN, "")
	assert.Equal(t, resp.ServiceNetworkID, "")
}

func Test_CreateOrUpdateServiceNetwork_MeshAlreadyExist_AssociateToNotAssociate(t *testing.T) {
	meshCreateInput := latticemodel.ServiceNetwork{
		Spec: latticemodel.ServiceNetworkSpec{
			Name:           "test",
			Account:        "123456789",
			AssociateToVPC: false,
		},
		Status: &latticemodel.ServiceNetworkStatus{ServiceNetworkARN: "", ServiceNetworkID: ""},
	}
	meshId := "12345678912345678912"
	meshArn := "12345678912345678912"
	name := "test"
	item := vpclattice.ServiceNetworkSummary{
		Arn:  &meshArn,
		Id:   &meshId,
		Name: &name,
	}

	status := vpclattice.ServiceNetworkVpcAssociationStatusActive
	items := vpclattice.ServiceNetworkVpcAssociationSummary{
		ServiceNetworkArn:  &meshArn,
		ServiceNetworkId:   &meshId,
		ServiceNetworkName: &meshId,
		Status:             &status,
		VpcId:              &config.VpcID,
	}
	statusServiceNetworkVPCOutput := []*vpclattice.ServiceNetworkVpcAssociationSummary{&items}

	c := gomock.NewController(t)
	defer c.Finish()
	ctx := context.TODO()
	mockLattice := mocks.NewMockLattice(c)
	mockCloud := mocks_aws.NewMockCloud(c)
	mockLattice.EXPECT().FindServiceNetwork(ctx, gomock.Any(), gomock.Any()).Return(
		&mocks.ServiceNetworkInfo{
			SvcNetwork: item,
			Tags:       nil,
		}, nil)
	mockLattice.EXPECT().ListServiceNetworkVpcAssociationsAsList(ctx, gomock.Any()).Return(statusServiceNetworkVPCOutput, nil)
	deleteInProgressStatus := vpclattice.ServiceNetworkVpcAssociationStatusDeleteInProgress
	deleteServiceNetworkVpcAssociationOutput := &vpclattice.DeleteServiceNetworkVpcAssociationOutput{Status: &deleteInProgressStatus}

	mockLattice.EXPECT().DeleteServiceNetworkVpcAssociationWithContext(ctx, gomock.Any()).Return(deleteServiceNetworkVpcAssociationOutput, nil)

	mockCloud.EXPECT().Lattice().Return(mockLattice).AnyTimes()

	meshManager := NewDefaultServiceNetworkManager(gwlog.FallbackLogger, mockCloud)
	_, err := meshManager.CreateOrUpdate(ctx, &meshCreateInput)

	assert.Equal(t, err, errors.New(LATTICE_RETRY))

}

// ServiceNetwork already exists, association is ServiceNetworkVpcAssociationStatusCreateFailed.

func Test_CreateOrUpdateServiceNetwork_MeshAlreadyExist_ServiceNetworkVpcAssociationStatusCreateFailed(t *testing.T) {
	meshCreateInput := latticemodel.ServiceNetwork{
		Spec: latticemodel.ServiceNetworkSpec{
			Name:           "test",
			Account:        "123456789",
			AssociateToVPC: true,
		},
		Status: &latticemodel.ServiceNetworkStatus{ServiceNetworkARN: "", ServiceNetworkID: ""},
	}
	meshId := "12345678912345678912"
	meshArn := "12345678912345678912"
	name := "test"
	item := vpclattice.ServiceNetworkSummary{
		Arn:  &meshArn,
		Id:   &meshId,
		Name: &name,
	}

	status := vpclattice.ServiceNetworkVpcAssociationStatusCreateFailed
	items := vpclattice.ServiceNetworkVpcAssociationSummary{
		ServiceNetworkArn:  &meshArn,
		ServiceNetworkId:   &meshId,
		ServiceNetworkName: &meshId,
		Status:             &status,
		VpcId:              &config.VpcID,
	}
	statusServiceNetworkVPCOutput := []*vpclattice.ServiceNetworkVpcAssociationSummary{&items}

	associationStatus := vpclattice.ServiceNetworkVpcAssociationStatusActive
	createServiceNetworkVPCAssociationOutput := &vpclattice.CreateServiceNetworkVpcAssociationOutput{
		Status: &associationStatus,
	}
	createServiceNetworkVpcAssociationInput := &vpclattice.CreateServiceNetworkVpcAssociationInput{
		ServiceNetworkIdentifier: &meshId,
		VpcIdentifier:            &config.VpcID,
	}

	c := gomock.NewController(t)
	defer c.Finish()
	ctx := context.TODO()
	mockLattice := mocks.NewMockLattice(c)
	mockCloud := mocks_aws.NewMockCloud(c)
	mockLattice.EXPECT().ListServiceNetworkVpcAssociationsAsList(ctx, gomock.Any()).Return(statusServiceNetworkVPCOutput, nil)
	snTagsOuput := &vpclattice.ListTagsForResourceOutput{
		Tags: make(map[string]*string),
	}
	snTagsOuput.Tags[latticemodel.K8SServiceNetworkOwnedByVPC] = &config.VpcID
	mockLattice.EXPECT().FindServiceNetwork(ctx, gomock.Any(), gomock.Any()).Return(
		&mocks.ServiceNetworkInfo{
			SvcNetwork: item,
			Tags:       snTagsOuput.Tags,
		}, nil)
	mockLattice.EXPECT().CreateServiceNetworkVpcAssociationWithContext(ctx, createServiceNetworkVpcAssociationInput).Return(createServiceNetworkVPCAssociationOutput, nil)
	mockCloud.EXPECT().Lattice().Return(mockLattice).AnyTimes()

	meshManager := NewDefaultServiceNetworkManager(gwlog.FallbackLogger, mockCloud)
	resp, err := meshManager.CreateOrUpdate(ctx, &meshCreateInput)

	assert.Nil(t, err)
	assert.Equal(t, resp.ServiceNetworkARN, meshArn)
	assert.Equal(t, resp.ServiceNetworkID, meshId)
}

// ServiceNetwork already exists, associated with other VPC
func Test_CreateOrUpdateServiceNetwork_MeshAlreadyExist_MeshAssociatedWithOtherVPC(t *testing.T) {
	meshCreateInput := latticemodel.ServiceNetwork{
		Spec: latticemodel.ServiceNetworkSpec{
			Name:           "test",
			Account:        "123456789",
			AssociateToVPC: true,
		},
		Status: &latticemodel.ServiceNetworkStatus{ServiceNetworkARN: "", ServiceNetworkID: ""},
	}
	meshId := "12345678912345678912"
	meshArn := "12345678912345678912"
	name := "test"
	vpcId := "123445677"
	item := vpclattice.ServiceNetworkSummary{
		Arn:  &meshArn,
		Id:   &meshId,
		Name: &name,
	}

	status := vpclattice.ServiceNetworkVpcAssociationStatusCreateFailed
	items := vpclattice.ServiceNetworkVpcAssociationSummary{
		ServiceNetworkArn:  &meshArn,
		ServiceNetworkId:   &meshId,
		ServiceNetworkName: &meshId,
		Status:             &status,
		VpcId:              &vpcId,
	}
	statusServiceNetworkVPCOutput := []*vpclattice.ServiceNetworkVpcAssociationSummary{&items}

	associationStatus := vpclattice.ServiceNetworkVpcAssociationStatusActive
	createServiceNetworkVPCAssociationOutput := &vpclattice.CreateServiceNetworkVpcAssociationOutput{
		Status: &associationStatus,
	}
	createServiceNetworkVpcAssociationInput := &vpclattice.CreateServiceNetworkVpcAssociationInput{
		ServiceNetworkIdentifier: &meshId,
		VpcIdentifier:            &config.VpcID,
	}

	c := gomock.NewController(t)
	defer c.Finish()
	ctx := context.TODO()
	mockLattice := mocks.NewMockLattice(c)
	mockCloud := mocks_aws.NewMockCloud(c)
	mockLattice.EXPECT().ListServiceNetworkVpcAssociationsAsList(ctx, gomock.Any()).Return(statusServiceNetworkVPCOutput, nil)
	snTagsOuput := &vpclattice.ListTagsForResourceOutput{
		Tags: make(map[string]*string),
	}
	dummy_vpc := "dummy-vpc-id"
	snTagsOuput.Tags[latticemodel.K8SServiceNetworkOwnedByVPC] = &dummy_vpc
	mockLattice.EXPECT().FindServiceNetwork(ctx, gomock.Any(), gomock.Any()).Return(
		&mocks.ServiceNetworkInfo{
			SvcNetwork: item,
			Tags:       snTagsOuput.Tags,
		}, nil)
	mockLattice.EXPECT().CreateServiceNetworkVpcAssociationWithContext(ctx, createServiceNetworkVpcAssociationInput).Return(createServiceNetworkVPCAssociationOutput, nil)
	mockCloud.EXPECT().Lattice().Return(mockLattice).AnyTimes()

	meshManager := NewDefaultServiceNetworkManager(gwlog.FallbackLogger, mockCloud)
	resp, err := meshManager.CreateOrUpdate(ctx, &meshCreateInput)

	assert.Nil(t, err)
	assert.Equal(t, resp.ServiceNetworkARN, meshArn)
	assert.Equal(t, resp.ServiceNetworkID, meshId)
}

// ServiceNetwork does not exists, association is ServiceNetworkVpcAssociationStatusFailed.
func Test_CreateOrUpdateServiceNetwork_MeshNotExist_ServiceNetworkVpcAssociationStatusFailed(t *testing.T) {
	CreateInput := latticemodel.ServiceNetwork{
		Spec: latticemodel.ServiceNetworkSpec{
			Name:           "test",
			Account:        "123456789",
			AssociateToVPC: true,
		},
		Status: &latticemodel.ServiceNetworkStatus{ServiceNetworkARN: "", ServiceNetworkID: ""},
	}
	meshId := "12345678912345678912"
	meshArn := "12345678912345678912"
	name := "test"

	associationStatus := vpclattice.ServiceNetworkVpcAssociationStatusCreateFailed
	createServiceNetworkVPCAssociationOutput := &vpclattice.CreateServiceNetworkVpcAssociationOutput{
		Status: &associationStatus,
	}
	meshCreateOutput := &vpclattice.CreateServiceNetworkOutput{
		Arn:  &meshArn,
		Id:   &meshId,
		Name: &name,
	}
	meshCreateInput := &vpclattice.CreateServiceNetworkInput{
		Name: &name,
		Tags: make(map[string]*string),
	}
	meshCreateInput.Tags[latticemodel.K8SServiceNetworkOwnedByVPC] = &config.VpcID

	createServiceNetworkVpcAssociationInput := &vpclattice.CreateServiceNetworkVpcAssociationInput{
		ServiceNetworkIdentifier: &meshId,
		VpcIdentifier:            &config.VpcID,
	}

	c := gomock.NewController(t)
	defer c.Finish()
	ctx := context.TODO()
	mockLattice := mocks.NewMockLattice(c)
	mockCloud := mocks_aws.NewMockCloud(c)
	mockLattice.EXPECT().FindServiceNetwork(ctx, gomock.Any(), gomock.Any()).Return(nil, nil)
	mockLattice.EXPECT().CreateServiceNetworkWithContext(ctx, meshCreateInput).Return(meshCreateOutput, nil)
	mockLattice.EXPECT().CreateServiceNetworkVpcAssociationWithContext(ctx, createServiceNetworkVpcAssociationInput).Return(createServiceNetworkVPCAssociationOutput, nil)
	mockCloud.EXPECT().Lattice().Return(mockLattice).AnyTimes()

	meshManager := NewDefaultServiceNetworkManager(gwlog.FallbackLogger, mockCloud)
	resp, err := meshManager.CreateOrUpdate(ctx, &CreateInput)

	assert.NotNil(t, err)
	assert.Equal(t, err, errors.New(LATTICE_RETRY))
	assert.Equal(t, resp.ServiceNetworkARN, "")
	assert.Equal(t, resp.ServiceNetworkID, "")
}

// ServiceNetwork does not exists, association is ServiceNetworkVpcAssociationStatusCreateInProgress.
func Test_CreateOrUpdateServiceNetwork_MeshNOTExist_ServiceNetworkVpcAssociationStatusCreateInProgress(t *testing.T) {
	CreateInput := latticemodel.ServiceNetwork{
		Spec: latticemodel.ServiceNetworkSpec{
			Name:           "test",
			Account:        "123456789",
			AssociateToVPC: true,
		},
		Status: &latticemodel.ServiceNetworkStatus{ServiceNetworkARN: "", ServiceNetworkID: ""},
	}
	meshId := "12345678912345678912"
	meshArn := "12345678912345678912"
	name := "test"
	associationStatus := vpclattice.ServiceNetworkVpcAssociationStatusCreateInProgress
	createServiceNetworkVPCAssociationOutput := &vpclattice.CreateServiceNetworkVpcAssociationOutput{
		Status: &associationStatus,
	}
	meshCreateOutput := &vpclattice.CreateServiceNetworkOutput{
		Arn:  &meshArn,
		Id:   &meshId,
		Name: &name,
	}
	meshCreateInput := &vpclattice.CreateServiceNetworkInput{
		Name: &name,
		Tags: make(map[string]*string),
	}
	meshCreateInput.Tags[latticemodel.K8SServiceNetworkOwnedByVPC] = &config.VpcID
	createServiceNetworkVpcAssociationInput := &vpclattice.CreateServiceNetworkVpcAssociationInput{
		ServiceNetworkIdentifier: &meshId,
		VpcIdentifier:            &config.VpcID,
	}

	c := gomock.NewController(t)
	defer c.Finish()
	ctx := context.TODO()
	mockLattice := mocks.NewMockLattice(c)
	mockCloud := mocks_aws.NewMockCloud(c)
	mockLattice.EXPECT().FindServiceNetwork(ctx, gomock.Any(), gomock.Any()).Return(nil, nil)
	mockLattice.EXPECT().CreateServiceNetworkWithContext(ctx, meshCreateInput).Return(meshCreateOutput, nil)
	mockLattice.EXPECT().CreateServiceNetworkVpcAssociationWithContext(ctx, createServiceNetworkVpcAssociationInput).Return(createServiceNetworkVPCAssociationOutput, nil)
	mockCloud.EXPECT().Lattice().Return(mockLattice).AnyTimes()

	meshManager := NewDefaultServiceNetworkManager(gwlog.FallbackLogger, mockCloud)
	resp, err := meshManager.CreateOrUpdate(ctx, &CreateInput)

	assert.NotNil(t, err)
	assert.Equal(t, err, errors.New(LATTICE_RETRY))
	assert.Equal(t, resp.ServiceNetworkARN, "")
	assert.Equal(t, resp.ServiceNetworkID, "")
}

// ServiceNetwork does not exists, association is ServiceNetworkVpcAssociationStatusDeleteInProgress.
func Test_CreateOrUpdateServiceNetwork_MeshNotExist_ServiceNetworkVpcAssociationStatusDeleteInProgress(t *testing.T) {
	CreateInput := latticemodel.ServiceNetwork{
		Spec: latticemodel.ServiceNetworkSpec{
			Name:           "test",
			Account:        "123456789",
			AssociateToVPC: true,
		},
		Status: &latticemodel.ServiceNetworkStatus{ServiceNetworkARN: "", ServiceNetworkID: ""},
	}
	meshId := "12345678912345678912"
	meshArn := "12345678912345678912"
	name := "test"
	associationStatus := vpclattice.ServiceNetworkVpcAssociationStatusDeleteInProgress
	createServiceNetworkVPCAssociationOutput := &vpclattice.CreateServiceNetworkVpcAssociationOutput{
		Status: &associationStatus,
	}
	meshCreateOutput := &vpclattice.CreateServiceNetworkOutput{
		Arn:  &meshArn,
		Id:   &meshId,
		Name: &name,
	}
	meshCreateInput := &vpclattice.CreateServiceNetworkInput{
		Name: &name,
		Tags: make(map[string]*string),
	}
	meshCreateInput.Tags[latticemodel.K8SServiceNetworkOwnedByVPC] = &config.VpcID
	createServiceNetworkVpcAssociationInput := &vpclattice.CreateServiceNetworkVpcAssociationInput{
		ServiceNetworkIdentifier: &meshId,
		VpcIdentifier:            &config.VpcID,
	}

	c := gomock.NewController(t)
	defer c.Finish()
	ctx := context.TODO()
	mockLattice := mocks.NewMockLattice(c)
	mockCloud := mocks_aws.NewMockCloud(c)
	mockLattice.EXPECT().FindServiceNetwork(ctx, gomock.Any(), gomock.Any()).Return(nil, nil)
	mockLattice.EXPECT().CreateServiceNetworkWithContext(ctx, meshCreateInput).Return(meshCreateOutput, nil)
	mockLattice.EXPECT().CreateServiceNetworkVpcAssociationWithContext(ctx, createServiceNetworkVpcAssociationInput).Return(createServiceNetworkVPCAssociationOutput, nil)
	mockCloud.EXPECT().Lattice().Return(mockLattice).AnyTimes()

	meshManager := NewDefaultServiceNetworkManager(gwlog.FallbackLogger, mockCloud)
	resp, err := meshManager.CreateOrUpdate(ctx, &CreateInput)

	assert.NotNil(t, err)
	assert.Equal(t, err, errors.New(LATTICE_RETRY))
	assert.Equal(t, resp.ServiceNetworkARN, "")
	assert.Equal(t, resp.ServiceNetworkID, "")
}

// ServiceNetwork does not exists, association returns Error.
func Test_CreateOrUpdateServiceNetwork_MeshNotExist_ServiceNetworkVpcAssociationReturnsError(t *testing.T) {
	CreateInput := latticemodel.ServiceNetwork{
		Spec: latticemodel.ServiceNetworkSpec{
			Name:           "test",
			Account:        "123456789",
			AssociateToVPC: true,
		},
		Status: &latticemodel.ServiceNetworkStatus{ServiceNetworkARN: "", ServiceNetworkID: ""},
	}
	meshId := "12345678912345678912"
	meshArn := "12345678912345678912"
	name := "test"
	createServiceNetworkVPCAssociationOutput := &vpclattice.CreateServiceNetworkVpcAssociationOutput{}
	meshCreateOutput := &vpclattice.CreateServiceNetworkOutput{
		Arn:  &meshArn,
		Id:   &meshId,
		Name: &name,
	}
	meshCreateInput := &vpclattice.CreateServiceNetworkInput{
		Name: &name,
		Tags: make(map[string]*string),
	}
	meshCreateInput.Tags[latticemodel.K8SServiceNetworkOwnedByVPC] = &config.VpcID
	createServiceNetworkVpcAssociationInput := &vpclattice.CreateServiceNetworkVpcAssociationInput{
		ServiceNetworkIdentifier: &meshId,
		VpcIdentifier:            &config.VpcID,
	}

	c := gomock.NewController(t)
	defer c.Finish()
	ctx := context.TODO()
	mockLattice := mocks.NewMockLattice(c)
	mockCloud := mocks_aws.NewMockCloud(c)
	mockLattice.EXPECT().FindServiceNetwork(ctx, gomock.Any(), gomock.Any()).Return(nil, nil)
	mockLattice.EXPECT().CreateServiceNetworkWithContext(ctx, meshCreateInput).Return(meshCreateOutput, nil)
	mockLattice.EXPECT().CreateServiceNetworkVpcAssociationWithContext(ctx, createServiceNetworkVpcAssociationInput).Return(createServiceNetworkVPCAssociationOutput, errors.New("ERROR"))
	mockCloud.EXPECT().Lattice().Return(mockLattice).AnyTimes()

	meshManager := NewDefaultServiceNetworkManager(gwlog.FallbackLogger, mockCloud)
	resp, err := meshManager.CreateOrUpdate(ctx, &CreateInput)

	assert.NotNil(t, err)
	assert.Equal(t, err, errors.New("ERROR"))
	assert.Equal(t, resp.ServiceNetworkARN, "")
	assert.Equal(t, resp.ServiceNetworkID, "")
}

// Mesh does not exist and failed to create.
func Test_CreateMesh_MeshNotExist_MeshCreateFailed(t *testing.T) {
	CreateInput := latticemodel.ServiceNetwork{
		Spec: latticemodel.ServiceNetworkSpec{
			Name:    "test",
			Account: "123456789",
		},
		Status: &latticemodel.ServiceNetworkStatus{ServiceNetworkARN: "", ServiceNetworkID: ""},
	}
	arn := "12345678912345678912"
	id := "12345678912345678912"
	name := "test"
	meshCreateOutput := &vpclattice.CreateServiceNetworkOutput{
		Arn:  &arn,
		Id:   &id,
		Name: &name,
	}
	meshCreateInput := &vpclattice.CreateServiceNetworkInput{
		Name: &name,
		Tags: make(map[string]*string),
	}

	meshCreateInput.Tags[latticemodel.K8SServiceNetworkOwnedByVPC] = &config.VpcID

	c := gomock.NewController(t)
	defer c.Finish()
	ctx := context.TODO()
	mockLattice := mocks.NewMockLattice(c)
	mockCloud := mocks_aws.NewMockCloud(c)
	mockLattice.EXPECT().FindServiceNetwork(ctx, gomock.Any(), gomock.Any()).Return(nil, nil)
	mockLattice.EXPECT().CreateServiceNetworkWithContext(ctx, meshCreateInput).Return(meshCreateOutput, errors.New("ERROR"))
	mockCloud.EXPECT().Lattice().Return(mockLattice).AnyTimes()

	meshManager := NewDefaultServiceNetworkManager(gwlog.FallbackLogger, mockCloud)
	resp, err := meshManager.CreateOrUpdate(ctx, &CreateInput)

	assert.NotNil(t, err)
	assert.Equal(t, err, errors.New("ERROR"))
	assert.Equal(t, resp.ServiceNetworkARN, "")
	assert.Equal(t, resp.ServiceNetworkID, "")
}

func Test_DeleteMesh_MeshNotExist(t *testing.T) {

	c := gomock.NewController(t)
	defer c.Finish()
	ctx := context.TODO()
	mockLattice := mocks.NewMockLattice(c)
	mockCloud := mocks_aws.NewMockCloud(c)
	mockLattice.EXPECT().FindServiceNetwork(ctx, gomock.Any(), gomock.Any()).Return(nil, &mocks.NotFoundError{})
	mockCloud.EXPECT().Lattice().Return(mockLattice).AnyTimes()

	meshManager := NewDefaultServiceNetworkManager(gwlog.FallbackLogger, mockCloud)
	err := meshManager.Delete(ctx, "test")

	assert.Nil(t, err)
}

// delete a service network, which has no association and also was created by this VPC
func Test_DeleteMesh_MeshExistsNoAssociation(t *testing.T) {
	arn := "123456789"
	id := "123456789"
	name := "test"
	item := vpclattice.ServiceNetworkSummary{
		Arn:  &arn,
		Id:   &id,
		Name: &name,
	}

	statusServiceNetworkVPCOutput := []*vpclattice.ServiceNetworkVpcAssociationSummary{}

	deleteMeshOutput := &vpclattice.DeleteServiceNetworkOutput{}
	deleteMeshInout := &vpclattice.DeleteServiceNetworkInput{ServiceNetworkIdentifier: &id}

	c := gomock.NewController(t)
	defer c.Finish()
	ctx := context.TODO()
	mockLattice := mocks.NewMockLattice(c)
	mockCloud := mocks_aws.NewMockCloud(c)
	mockLattice.EXPECT().ListServiceNetworkVpcAssociationsAsList(ctx, gomock.Any()).Return(statusServiceNetworkVPCOutput, nil)

	snTagsOuput := &vpclattice.ListTagsForResourceOutput{
		Tags: make(map[string]*string),
	}
	snTagsOuput.Tags[latticemodel.K8SServiceNetworkOwnedByVPC] = &config.VpcID
	mockLattice.EXPECT().FindServiceNetwork(ctx, gomock.Any(), gomock.Any()).Return(
		&mocks.ServiceNetworkInfo{
			SvcNetwork: item,
			Tags:       snTagsOuput.Tags,
		}, nil)
	mockLattice.EXPECT().DeleteServiceNetworkWithContext(ctx, deleteMeshInout).Return(deleteMeshOutput, nil)
	mockCloud.EXPECT().Lattice().Return(mockLattice).AnyTimes()

	meshManager := NewDefaultServiceNetworkManager(gwlog.FallbackLogger, mockCloud)
	err := meshManager.Delete(ctx, "test")

	assert.Nil(t, err)
}

// Deleting a service netwrok, when
// * the service network is associated with current VPC
// * and it is this VPC creates this service network
func Test_DeleteMesh_MeshExistsAssociatedWithVPC_Deleting(t *testing.T) {
	arn := "123456789"
	id := "123456789"
	name := "test"
	item := vpclattice.ServiceNetworkSummary{
		Arn:  &arn,
		Id:   &id,
		Name: &name,
	}

	associationArn := "123456789"
	associationID := "123456789"
	associationStatus := vpclattice.ServiceNetworkVpcAssociationStatusActive
	associationVPCId := config.VpcID
	itemAssociation := vpclattice.ServiceNetworkVpcAssociationSummary{
		Arn:                &associationArn,
		Id:                 &associationID,
		ServiceNetworkArn:  &arn,
		ServiceNetworkId:   &id,
		ServiceNetworkName: &name,
		Status:             &associationStatus,
		VpcId:              &associationVPCId,
	}
	statusServiceNetworkVPCOutput := []*vpclattice.ServiceNetworkVpcAssociationSummary{&itemAssociation}

	deleteInProgressStatus := vpclattice.ServiceNetworkVpcAssociationStatusDeleteInProgress
	deleteServiceNetworkVpcAssociationOutput := &vpclattice.DeleteServiceNetworkVpcAssociationOutput{Status: &deleteInProgressStatus}
	deleteServiceNetworkVpcAssociationInput := &vpclattice.DeleteServiceNetworkVpcAssociationInput{ServiceNetworkVpcAssociationIdentifier: &associationID}

	c := gomock.NewController(t)
	defer c.Finish()
	ctx := context.TODO()
	mockLattice := mocks.NewMockLattice(c)
	mockCloud := mocks_aws.NewMockCloud(c)
	mockLattice.EXPECT().ListServiceNetworkVpcAssociationsAsList(ctx, gomock.Any()).Return(statusServiceNetworkVPCOutput, nil)

	snTagsOuput := &vpclattice.ListTagsForResourceOutput{
		Tags: make(map[string]*string),
	}
	snTagsOuput.Tags[latticemodel.K8SServiceNetworkOwnedByVPC] = &config.VpcID
	mockLattice.EXPECT().FindServiceNetwork(ctx, gomock.Any(), gomock.Any()).Return(
		&mocks.ServiceNetworkInfo{
			SvcNetwork: item,
			Tags:       snTagsOuput.Tags,
		}, nil)
	mockLattice.EXPECT().DeleteServiceNetworkVpcAssociationWithContext(ctx, deleteServiceNetworkVpcAssociationInput).Return(deleteServiceNetworkVpcAssociationOutput, nil)
	mockCloud.EXPECT().Lattice().Return(mockLattice).AnyTimes()

	meshManager := NewDefaultServiceNetworkManager(gwlog.FallbackLogger, mockCloud)
	err := meshManager.Delete(ctx, "test")

	assert.NotNil(t, err)
	assert.Equal(t, err, errors.New(LATTICE_RETRY))
}

func Test_DeleteMesh_MeshExistsAssociatedWithOtherVPC(t *testing.T) {
	arn := "123456789"
	id := "123456789"
	name := "test"
	item := vpclattice.ServiceNetworkSummary{
		Arn:  &arn,
		Id:   &id,
		Name: &name,
	}

	associationArn := "123456789"
	associationID := "123456789"
	associationStatus := vpclattice.ServiceNetworkVpcAssociationStatusActive
	associationVPCId := "other-vpc-id"
	itemAssociation := vpclattice.ServiceNetworkVpcAssociationSummary{
		Arn:                &associationArn,
		Id:                 &associationID,
		ServiceNetworkArn:  &arn,
		ServiceNetworkId:   &id,
		ServiceNetworkName: &name,
		Status:             &associationStatus,
		VpcId:              &associationVPCId,
	}
	statusServiceNetworkVPCOutput := []*vpclattice.ServiceNetworkVpcAssociationSummary{&itemAssociation}

	c := gomock.NewController(t)
	defer c.Finish()
	ctx := context.TODO()
	mockLattice := mocks.NewMockLattice(c)
	mockCloud := mocks_aws.NewMockCloud(c)
	mockLattice.EXPECT().ListServiceNetworkVpcAssociationsAsList(ctx, gomock.Any()).Return(statusServiceNetworkVPCOutput, nil)

	snTagsOuput := &vpclattice.ListTagsForResourceOutput{
		Tags: make(map[string]*string),
	}
	snTagsOuput.Tags[latticemodel.K8SServiceNetworkOwnedByVPC] = &config.VpcID
	mockLattice.EXPECT().FindServiceNetwork(ctx, gomock.Any(), gomock.Any()).Return(
		&mocks.ServiceNetworkInfo{
			SvcNetwork: item,
			Tags:       snTagsOuput.Tags,
		}, nil)
	mockCloud.EXPECT().Lattice().Return(mockLattice).AnyTimes()

	meshManager := NewDefaultServiceNetworkManager(gwlog.FallbackLogger, mockCloud)
	err := meshManager.Delete(ctx, "test")

	assert.NotNil(t, err)
	assert.Equal(t, err, errors.New(LATTICE_RETRY))
}

func Test_DeleteMesh_MeshExistsAssociatedWithOtherVPC_NotCreatedByVPC(t *testing.T) {
	arn := "123456789"
	id := "123456789"
	name := "test"
	item := vpclattice.ServiceNetworkSummary{
		Arn:  &arn,
		Id:   &id,
		Name: &name,
	}

	associationArn := "123456789"
	associationID := "123456789"
	associationStatus := vpclattice.ServiceNetworkVpcAssociationStatusActive
	associationVPCId := "123456789"
	itemAssociation := vpclattice.ServiceNetworkVpcAssociationSummary{
		Arn:                &associationArn,
		Id:                 &associationID,
		ServiceNetworkArn:  &arn,
		ServiceNetworkId:   &id,
		ServiceNetworkName: &name,
		Status:             &associationStatus,
		VpcId:              &associationVPCId,
	}
	statusServiceNetworkVPCOutput := []*vpclattice.ServiceNetworkVpcAssociationSummary{&itemAssociation}

	c := gomock.NewController(t)
	defer c.Finish()
	ctx := context.TODO()
	mockLattice := mocks.NewMockLattice(c)
	mockCloud := mocks_aws.NewMockCloud(c)
	mockLattice.EXPECT().FindServiceNetwork(ctx, gomock.Any(), gomock.Any()).Return(
		&mocks.ServiceNetworkInfo{
			SvcNetwork: item,
			Tags:       nil,
		}, nil)
	mockLattice.EXPECT().ListServiceNetworkVpcAssociationsAsList(ctx, gomock.Any()).Return(statusServiceNetworkVPCOutput, nil)
	mockCloud.EXPECT().Lattice().Return(mockLattice).AnyTimes()

	meshManager := NewDefaultServiceNetworkManager(gwlog.FallbackLogger, mockCloud)
	err := meshManager.Delete(ctx, "test")

	assert.Nil(t, err)
}

func Test_DeleteMesh_MeshExistsAssociatedWithOtherVPC_CreatedByVPC(t *testing.T) {
	arn := "123456789"
	id := "123456789"
	name := "test"
	item := vpclattice.ServiceNetworkSummary{
		Arn:  &arn,
		Id:   &id,
		Name: &name,
	}

	associationArn := "123456789"
	associationID := "123456789"
	associationStatus := vpclattice.ServiceNetworkVpcAssociationStatusActive
	associationVPCId := "123456789"
	itemAssociation := vpclattice.ServiceNetworkVpcAssociationSummary{
		Arn:                &associationArn,
		Id:                 &associationID,
		ServiceNetworkArn:  &arn,
		ServiceNetworkId:   &id,
		ServiceNetworkName: &name,
		Status:             &associationStatus,
		VpcId:              &associationVPCId,
	}
	statusServiceNetworkVPCOutput := []*vpclattice.ServiceNetworkVpcAssociationSummary{&itemAssociation}

	c := gomock.NewController(t)
	defer c.Finish()
	ctx := context.TODO()
	mockLattice := mocks.NewMockLattice(c)
	mockCloud := mocks_aws.NewMockCloud(c)
	mockLattice.EXPECT().ListServiceNetworkVpcAssociationsAsList(ctx, gomock.Any()).Return(statusServiceNetworkVPCOutput, nil)
	snTagsOutput := &vpclattice.ListTagsForResourceOutput{
		Tags: make(map[string]*string),
	}
	snTagsOutput.Tags[latticemodel.K8SServiceNetworkOwnedByVPC] = &config.VpcID
	mockLattice.EXPECT().FindServiceNetwork(ctx, gomock.Any(), gomock.Any()).Return(
		&mocks.ServiceNetworkInfo{
			SvcNetwork: item,
			Tags:       snTagsOutput.Tags,
		}, nil)
	mockCloud.EXPECT().Lattice().Return(mockLattice).AnyTimes()

	meshManager := NewDefaultServiceNetworkManager(gwlog.FallbackLogger, mockCloud)
	err := meshManager.Delete(ctx, "test")

	assert.NotNil(t, err)
	assert.Equal(t, err, errors.New(LATTICE_RETRY))
}

func Test_ListMesh_MeshExists(t *testing.T) {
	arn := "123456789"
	id := "123456789"
	name1 := "test1"
	item1 := vpclattice.ServiceNetworkSummary{
		Arn:  &arn,
		Id:   &id,
		Name: &name1,
	}
	name2 := "test2"
	item2 := vpclattice.ServiceNetworkSummary{
		Arn:  &arn,
		Id:   &id,
		Name: &name2,
	}
	listServiceNetworkOutput := []*vpclattice.ServiceNetworkSummary{&item1, &item2}

	c := gomock.NewController(t)
	defer c.Finish()
	ctx := context.TODO()
	mockLattice := mocks.NewMockLattice(c)
	mockCloud := mocks_aws.NewMockCloud(c)
	mockLattice.EXPECT().ListServiceNetworksAsList(ctx, gomock.Any()).Return(listServiceNetworkOutput, nil)
	mockCloud.EXPECT().Lattice().Return(mockLattice)

	meshManager := NewDefaultServiceNetworkManager(gwlog.FallbackLogger, mockCloud)
	meshList, err := meshManager.List(ctx)

	assert.Nil(t, err)
	assert.Equal(t, meshList, []string{"test1", "test2"})
}

func Test_ListMesh_NoMesh(t *testing.T) {
	listServiceNetworkOutput := []*vpclattice.ServiceNetworkSummary{}

	c := gomock.NewController(t)
	defer c.Finish()
	ctx := context.TODO()
	mockLattice := mocks.NewMockLattice(c)
	mockCloud := mocks_aws.NewMockCloud(c)
	mockLattice.EXPECT().ListServiceNetworksAsList(ctx, gomock.Any()).Return(listServiceNetworkOutput, nil)
	mockCloud.EXPECT().Lattice().Return(mockLattice)

	meshManager := NewDefaultServiceNetworkManager(gwlog.FallbackLogger, mockCloud)
	meshList, err := meshManager.List(ctx)

	assert.Nil(t, err)
	assert.Equal(t, meshList, []string{})
}
