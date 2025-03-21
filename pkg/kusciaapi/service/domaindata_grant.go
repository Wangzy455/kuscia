// Copyright 2023 Ant Group Co., Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//nolint:dupl
package service

import (
	"context"
	"crypto/rsa"
	"fmt"
	"time"

	"google.golang.org/protobuf/types/known/anypb"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"

	"github.com/secretflow/kuscia/pkg/common"
	"github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	"github.com/secretflow/kuscia/pkg/kusciaapi/config"
	"github.com/secretflow/kuscia/pkg/kusciaapi/errorcode"
	"github.com/secretflow/kuscia/pkg/utils/nlog"
	"github.com/secretflow/kuscia/pkg/utils/resources"
	"github.com/secretflow/kuscia/pkg/web/utils"
	pberrorcode "github.com/secretflow/kuscia/proto/api/v1alpha1/errorcode"
	"github.com/secretflow/kuscia/proto/api/v1alpha1/kusciaapi"
)

type IDomainDataGrantService interface {
	CreateDomainDataGrant(ctx context.Context, request *kusciaapi.CreateDomainDataGrantRequest) *kusciaapi.CreateDomainDataGrantResponse
	QueryDomainDataGrant(ctx context.Context, request *kusciaapi.QueryDomainDataGrantRequest) *kusciaapi.QueryDomainDataGrantResponse
	UpdateDomainDataGrant(ctx context.Context, request *kusciaapi.UpdateDomainDataGrantRequest) *kusciaapi.UpdateDomainDataGrantResponse
	DeleteDomainDataGrant(ctx context.Context, request *kusciaapi.DeleteDomainDataGrantRequest) *kusciaapi.DeleteDomainDataGrantResponse
	BatchQueryDomainDataGrant(ctx context.Context, request *kusciaapi.BatchQueryDomainDataGrantRequest) *kusciaapi.BatchQueryDomainDataGrantResponse
	ListDomainDataGrant(ctx context.Context, request *kusciaapi.ListDomainDataGrantRequest) *kusciaapi.ListDomainDataGrantResponse
}

type domainDataGrantService struct {
	conf   *config.KusciaAPIConfig
	priKey *rsa.PrivateKey
}

func NewDomainDataGrantService(config *config.KusciaAPIConfig) IDomainDataGrantService {

	return &domainDataGrantService{
		conf:   config,
		priKey: config.DomainKey,
	}
}

func (s *domainDataGrantService) CreateDomainDataGrant(ctx context.Context, request *kusciaapi.CreateDomainDataGrantRequest) *kusciaapi.CreateDomainDataGrantResponse {
	// do validate
	if validateErr := validateCreateDomainDataGrantRequest(request); validateErr != nil {
		return &kusciaapi.CreateDomainDataGrantResponse{
			Status: utils.BuildErrorResponseStatus(pberrorcode.ErrorCode_KusciaAPIErrRequestValidate, validateErr.Error()),
		}
	}

	dd, err := s.conf.KusciaClient.KusciaV1alpha1().DomainDatas(request.DomainId).Get(ctx, request.DomaindataId, metav1.GetOptions{})
	if err != nil {
		return &kusciaapi.CreateDomainDataGrantResponse{
			Status: utils.BuildErrorResponseStatus(pberrorcode.ErrorCode_KusciaAPIErrRequestValidate, fmt.Sprintf("domaindata [%s] not exists", request.DomaindataId)),
		}
	}
	if request.DomaindatagrantId != "" {
		_, queryErr := s.conf.KusciaClient.KusciaV1alpha1().DomainDataGrants(request.DomainId).Get(ctx, request.DomaindatagrantId, metav1.GetOptions{})
		if queryErr == nil {
			return &kusciaapi.CreateDomainDataGrantResponse{
				Status: utils.BuildErrorResponseStatus(errorcode.GetDomainDataGrantErrorCode(queryErr, pberrorcode.ErrorCode_KusciaAPIErrCreateDomainDataGrant), fmt.Sprintf("CreateDomainDataGrant failed, because domaindatagrant %s is exist", request.DomaindatagrantId)),
			}
		}
	}

	dg := &v1alpha1.DomainDataGrant{}
	dg.Labels = map[string]string{}
	dg.OwnerReferences = append(dg.OwnerReferences, *metav1.NewControllerRef(dd, v1alpha1.SchemeGroupVersion.WithKind("DomainData")))
	s.convertData2Spec(&kusciaapi.DomainDataGrantData{
		Author:            request.DomainId,
		DomaindataId:      request.DomaindataId,
		DomaindatagrantId: request.DomaindatagrantId,
		GrantDomain:       request.GrantDomain,
		Limit:             request.Limit,
		Description:       request.Description,
		Signature:         request.Signature,
		DomainId:          request.DomainId,
	}, dg)

	dg, err = s.conf.KusciaClient.KusciaV1alpha1().DomainDataGrants(request.DomainId).Create(ctx, dg, metav1.CreateOptions{})
	if err != nil {
		nlog.Errorf("CreateDomainDataGrant failed, error:%s", err.Error())
		return &kusciaapi.CreateDomainDataGrantResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.CreateDomainDataGrantErrorCode(err, pberrorcode.ErrorCode_KusciaAPIErrCreateDomainDataGrant), err.Error()),
		}
	}
	return &kusciaapi.CreateDomainDataGrantResponse{
		Status: utils.BuildSuccessResponseStatus(),
		Data: &kusciaapi.CreateDomainDataGrantResponseData{
			DomaindatagrantId: dg.Name,
		},
	}
}

func (s *domainDataGrantService) QueryDomainDataGrant(ctx context.Context, request *kusciaapi.QueryDomainDataGrantRequest) *kusciaapi.QueryDomainDataGrantResponse {

	dg, err := s.conf.KusciaClient.KusciaV1alpha1().DomainDataGrants(request.DomainId).Get(ctx, request.DomaindatagrantId, metav1.GetOptions{})
	if err != nil {
		nlog.Errorf("Query DomainDataGrant failed, error:%s", err.Error())
		return &kusciaapi.QueryDomainDataGrantResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.GetDomainDataGrantErrorCode(err, pberrorcode.ErrorCode_KusciaAPIErrQueryDomainDataGrant), err.Error()),
		}
	}

	grant := &kusciaapi.DomainDataGrant{}
	s.convertSpec2Data(dg, grant)
	return &kusciaapi.QueryDomainDataGrantResponse{
		Status: utils.BuildSuccessResponseStatus(),
		Data:   grant,
	}
}

func (s *domainDataGrantService) BatchQueryDomainDataGrant(ctx context.Context, request *kusciaapi.BatchQueryDomainDataGrantRequest) *kusciaapi.BatchQueryDomainDataGrantResponse {
	data := []*kusciaapi.DomainDataGrant{}
	if len(request.Data) == 0 {
		// empty query data
		return &kusciaapi.BatchQueryDomainDataGrantResponse{
			Status: utils.BuildErrorResponseStatus(pberrorcode.ErrorCode_KusciaAPIErrRequestValidate, "BatchQueryDomainDataGrant.data can't be null"),
			Data:   data,
		}
	}
	emptyRequestCount := 0
	errorResponseCount := 0
	detail := make([]*anypb.Any, len(request.Data))
	for i, req := range request.Data {
		if req.DomainId == "" || req.DomaindatagrantId == "" {
			emptyRequestCount++
			data = append(data, &kusciaapi.DomainDataGrant{})
			tempDetail, err := anypb.New(utils.BuildErrorResponseStatus(pberrorcode.ErrorCode_KusciaAPIErrRequestValidate, "QueryDomainDataGrantRequestData can't be null"))
			if err != nil {
				tempDetail = nil
			}
			detail[i] = tempDetail
			continue
		}
		resp := s.QueryDomainDataGrant(ctx, &kusciaapi.QueryDomainDataGrantRequest{
			DomainId:          req.DomainId,
			DomaindatagrantId: req.DomaindatagrantId,
		})
		if resp.Status.Code != 0 {
			errorResponseCount++
		}
		data = append(data, resp.Data)
		tempDetail, err := anypb.New(resp.Status)
		if err != nil {
			tempDetail = nil
		}
		detail[i] = tempDetail
	}

	var ret *kusciaapi.BatchQueryDomainDataGrantResponse

	if emptyRequestCount != 0 || errorResponseCount != 0 {
		errMsg := fmt.Sprintf("BatchQuery has error. Empty/Error/Total(%d/%d/%d)", emptyRequestCount, errorResponseCount, len(request.Data))
		ret = &kusciaapi.BatchQueryDomainDataGrantResponse{
			Status: utils.BuildErrorResponseStatus(pberrorcode.ErrorCode_KusciaAPIErrQueryDomainDataGrant, errMsg),
			Data:   data,
		}
		// add detail empty & error message to status.detail
		ret.Status.Details = detail
	} else {
		ret = &kusciaapi.BatchQueryDomainDataGrantResponse{
			Status: utils.BuildSuccessResponseStatus(),
			Data:   data,
		}
	}

	return ret
}

func (s *domainDataGrantService) UpdateDomainDataGrant(ctx context.Context, request *kusciaapi.UpdateDomainDataGrantRequest) *kusciaapi.UpdateDomainDataGrantResponse {

	if request.DomaindataId == "" {
		return &kusciaapi.UpdateDomainDataGrantResponse{
			Status: utils.BuildErrorResponseStatus(pberrorcode.ErrorCode_KusciaAPIErrRequestValidate, "domaindataid can't be null"),
		}
	}
	dd, err := s.conf.KusciaClient.KusciaV1alpha1().DomainDatas(request.DomainId).Get(ctx, request.DomaindataId, metav1.GetOptions{})
	if err != nil {
		return &kusciaapi.UpdateDomainDataGrantResponse{
			Status: utils.BuildErrorResponseStatus(pberrorcode.ErrorCode_KusciaAPIErrRequestValidate, "domaindata can't be found"),
		}
	}
	if request.GrantDomain == "" {
		return &kusciaapi.UpdateDomainDataGrantResponse{
			Status: utils.BuildErrorResponseStatus(pberrorcode.ErrorCode_KusciaAPIErrRequestValidate, "grantdomain can't be null"),
		}
	}
	if request.DomaindatagrantId == "" {
		return &kusciaapi.UpdateDomainDataGrantResponse{
			Status: utils.BuildErrorResponseStatus(pberrorcode.ErrorCode_KusciaAPIErrRequestValidate, "grantdomainid can't be null"),
		}
	}

	dg, err := s.conf.KusciaClient.KusciaV1alpha1().DomainDataGrants(request.DomainId).Get(ctx, request.DomaindatagrantId, metav1.GetOptions{})
	if err != nil {
		nlog.Errorf("Get DomainDataGrant failed, error:%s", err.Error())
		return &kusciaapi.UpdateDomainDataGrantResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.GetDomainDataGrantErrorCode(err, pberrorcode.ErrorCode_KusciaAPIErrQueryDomainDataGrant), err.Error()),
		}
	}

	// if domaindatagrant is exist, and try to modify domaindata id, show better fail reason
	if dg.Spec.DomainDataID != request.DomaindataId {
		return &kusciaapi.UpdateDomainDataGrantResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.GetDomainDataGrantErrorCode(err, pberrorcode.ErrorCode_KusciaAPIErrRequestValidate), "domaindataid can't be changed."),
		}
	}

	s.convertData2Spec(&kusciaapi.DomainDataGrantData{
		DomaindatagrantId: request.DomaindatagrantId,
		Author:            request.DomainId,
		DomaindataId:      request.DomaindataId,
		GrantDomain:       request.GrantDomain,
		Limit:             request.Limit,
		Description:       request.Description,
		Signature:         request.Signature,
		DomainId:          request.DomainId,
	}, dg)

	if dg.Labels == nil {
		dg.Labels = map[string]string{}
	}
	dg.OwnerReferences = append(dg.OwnerReferences, *metav1.NewControllerRef(dd, v1alpha1.SchemeGroupVersion.WithKind("DomainData")))

	_, err = s.conf.KusciaClient.KusciaV1alpha1().DomainDataGrants(request.DomainId).Update(ctx, dg, metav1.UpdateOptions{})
	if err != nil {
		nlog.Errorf("Update DomainDataGrant failed, error:%s", err.Error())
		return &kusciaapi.UpdateDomainDataGrantResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.GetDomainDataGrantErrorCode(err, pberrorcode.ErrorCode_KusciaAPIErrUpdateDomainDataGrant), err.Error()),
		}
	}
	return &kusciaapi.UpdateDomainDataGrantResponse{
		Status: utils.BuildSuccessResponseStatus(),
	}
}

func (s *domainDataGrantService) DeleteDomainDataGrant(ctx context.Context, request *kusciaapi.DeleteDomainDataGrantRequest) *kusciaapi.DeleteDomainDataGrantResponse {
	nlog.Warnf("Delete domainDataGrantId %s", request.DomaindatagrantId)
	err := s.conf.KusciaClient.KusciaV1alpha1().DomainDataGrants(request.DomainId).Delete(ctx, request.DomaindatagrantId, metav1.DeleteOptions{})
	if err != nil {
		nlog.Errorf("Delete domainDataGrantId:%s failed, detail:%s", request.DomaindatagrantId, err.Error())
		return &kusciaapi.DeleteDomainDataGrantResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.GetDomainDataGrantErrorCode(err, pberrorcode.ErrorCode_KusciaAPIErrDeleteDomainDataGrant), err.Error()),
		}
	}
	return &kusciaapi.DeleteDomainDataGrantResponse{
		Status: utils.BuildSuccessResponseStatus(),
	}
}

func (s *domainDataGrantService) ListDomainDataGrant(ctx context.Context, request *kusciaapi.ListDomainDataGrantRequest) *kusciaapi.ListDomainDataGrantResponse {
	// do validate
	if request.Data == nil || request.Data.DomainId == "" {
		return &kusciaapi.ListDomainDataGrantResponse{
			Status: utils.BuildErrorResponseStatus(pberrorcode.ErrorCode_KusciaAPIErrRequestValidate, "domain id can not be empty"),
		}
	}
	// construct label selector
	var (
		selector    fields.Selector
		selectorStr string
	)

	if request.Data.DomaindataVendor != "" {
		vendorSelector := fields.OneTermEqualSelector(common.LabelDomainDataGrantVendor, request.Data.DomaindataVendor)
		if selector != nil {
			selector = fields.AndSelectors(selector, vendorSelector)
		} else {
			selector = vendorSelector
		}
		selectorStr = selector.String()
	}
	if request.Data.GrantDomain != "" {
		vendorSelector := fields.OneTermEqualSelector(common.LabelDomainDataGrantDomain, request.Data.GrantDomain)
		if selector != nil {
			selector = fields.AndSelectors(selector, vendorSelector)
		} else {
			selector = vendorSelector
		}
		selectorStr = selector.String()
	}

	// get kuscia domain
	// todo support limit and continue
	dataList, err := s.conf.KusciaClient.KusciaV1alpha1().DomainDataGrants(request.Data.DomainId).List(ctx, metav1.ListOptions{
		TypeMeta:       metav1.TypeMeta{},
		LabelSelector:  selectorStr,
		TimeoutSeconds: nil,
		Limit:          0,
		Continue:       "",
	})
	if err != nil {
		nlog.Errorf("List DomainData failed, error:%s", err.Error())
		return &kusciaapi.ListDomainDataGrantResponse{
			Status: utils.BuildErrorResponseStatus(errorcode.GetDomainDataGrantErrorCode(err, pberrorcode.ErrorCode_KusciaAPIErrListDomainDataFailed), err.Error()),
		}
	}
	grantLists := make([]*kusciaapi.DomainDataGrant, len(dataList.Items))
	for i, v := range dataList.Items {
		grant := &kusciaapi.DomainDataGrant{}
		s.convertSpec2Data(&v, grant)
		grantLists[i] = grant
	}

	// build domain response
	return &kusciaapi.ListDomainDataGrantResponse{
		Status: utils.BuildSuccessResponseStatus(),
		Data: &kusciaapi.DomainDataGrantList{
			DomaindatagrantList: grantLists,
		},
	}
}

func (s *domainDataGrantService) convertData2Spec(data *kusciaapi.DomainDataGrantData, v *v1alpha1.DomainDataGrant) {
	var limit *v1alpha1.GrantLimit
	if data.Limit != nil {
		grantMode := []v1alpha1.GrantType{"normal"}
		limit = &v1alpha1.GrantLimit{
			FlowID:      data.Limit.FlowId,
			UseCount:    int(data.Limit.UseCount),
			Initiator:   data.Limit.Initiator,
			InputConfig: data.Limit.InputConfig,
			Components:  data.Limit.Components,
			GrantMode:   grantMode,
		}
		if data.Limit.ExpirationTime > 0 {
			mt := metav1.NewTime(time.Unix(data.Limit.ExpirationTime/int64(time.Second), data.Limit.ExpirationTime%int64(time.Second)))
			limit.ExpirationTime = &mt
		}
	}

	dgID := data.DomaindatagrantId
	if dgID == "" {
		dgID = common.GenDomainDataID("domaindatagrant")
	}
	v.Name = dgID
	if v.Labels == nil {
		v.Labels = map[string]string{}
	}

	v.Labels[common.LabelDomainDataGrantDomain] = data.GrantDomain

	v.Spec = v1alpha1.DomainDataGrantSpec{
		Author:       data.Author,
		DomainDataID: data.DomaindataId,
		Signature:    data.Signature,
		GrantDomain:  data.GrantDomain,
		Limit:        limit,
		Description:  data.Description,
	}
}

func (s *domainDataGrantService) convertSpec2Data(v *v1alpha1.DomainDataGrant, grant *kusciaapi.DomainDataGrant) {
	if grant.Data == nil {
		grant.Data = &kusciaapi.DomainDataGrantData{}
	}
	data := grant.Data
	data.Author = v.Spec.Author
	data.DomaindataId = v.Spec.DomainDataID
	data.DomaindatagrantId = v.Name
	data.GrantDomain = v.Spec.GrantDomain

	if v.Spec.Limit != nil {
		data.Limit = &kusciaapi.GrantLimit{
			Components:  v.Spec.Limit.Components,
			FlowId:      v.Spec.Limit.FlowID,
			UseCount:    int32(v.Spec.Limit.UseCount),
			InputConfig: v.Spec.Limit.InputConfig,
		}
		if v.Spec.Limit.ExpirationTime != nil {
			data.Limit.ExpirationTime = v.Spec.Limit.ExpirationTime.UnixNano()
		}
	}

	if grant.Status == nil {
		grant.Status = &kusciaapi.DomainDataGrantStatus{}
	}
	grant.Status.Phase = string(v.Status.Phase)
	grant.Status.Message = v.Status.Message
	grant.Status.Records = nil
	for _, r := range v.Status.UseRecords {
		grant.Status.Records = append(grant.Status.Records, &kusciaapi.UseRecord{
			UseTime:     r.UseTime.UnixNano(),
			GrantDomain: r.GrantDomain,
			Component:   r.Component,
			Output:      r.Output,
		})
	}
}

func validateCreateDomainDataGrantRequest(request *kusciaapi.CreateDomainDataGrantRequest) error {

	if request.GrantDomain == "" {
		return fmt.Errorf("grantdomain can't be null")
	}

	if request.GrantDomain == request.DomainId {
		return fmt.Errorf("grantdomain can't be self")
	}

	if request.DomaindataId == "" {
		return fmt.Errorf("domaindata can't be null")
	}
	// do k8s validate
	if err := resources.ValidateK8sName(request.DomainId, "domain_id"); err != nil {
		return err
	}

	if err := resources.ValidateK8sName(request.DomaindataId, "domaindata_id"); err != nil {
		return err
	}

	if request.GetDomaindatagrantId() != "" {
		return resources.ValidateK8sName(request.GetDomaindatagrantId(), "domaindatagrant_id")
	}
	return nil
}
