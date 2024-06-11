// Copyright 2023 Ant Group Co., Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package handler

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	kubefake "k8s.io/client-go/kubernetes/fake"

	kusciaapisv1alpha1 "github.com/secretflow/kuscia/pkg/crd/apis/kuscia/v1alpha1"
	"github.com/secretflow/kuscia/pkg/crd/clientset/versioned"
	kusciafake "github.com/secretflow/kuscia/pkg/crd/clientset/versioned/fake"
	kusciainformers "github.com/secretflow/kuscia/pkg/crd/informers/externalversions"
)

// all party accept
func setJobAllPartyAccept(job *kusciaapisv1alpha1.KusciaJob) {
	job.Status.Phase = kusciaapisv1alpha1.KusciaJobAwaitingApproval
	job.Status.ApproveStatus = map[string]kusciaapisv1alpha1.JobApprovePhase{}
	job.Status.ApproveStatus["alice"] = kusciaapisv1alpha1.JobAccepted
	job.Status.ApproveStatus["bob"] = kusciaapisv1alpha1.JobAccepted
}

// one party reject
func setJobOnePartyReject(job *kusciaapisv1alpha1.KusciaJob) {
	job.Status.Phase = kusciaapisv1alpha1.KusciaJobAwaitingApproval
	job.Status.ApproveStatus = map[string]kusciaapisv1alpha1.JobApprovePhase{}
	job.Status.ApproveStatus["alice"] = kusciaapisv1alpha1.JobAccepted
	job.Status.ApproveStatus["bob"] = kusciaapisv1alpha1.JobRejected
}

// only one party accept
func setJobOnlyOnePartyAccept(job *kusciaapisv1alpha1.KusciaJob) {
	job.Status.Phase = kusciaapisv1alpha1.KusciaJobAwaitingApproval
	job.Status.ApproveStatus = map[string]kusciaapisv1alpha1.JobApprovePhase{}
	job.Status.ApproveStatus["alice"] = kusciaapisv1alpha1.JobAccepted
}

// all party null
func setJobAllPartyNil(job *kusciaapisv1alpha1.KusciaJob) {
	job.Status.Phase = kusciaapisv1alpha1.KusciaJobAwaitingApproval
	job.Status.ApproveStatus = nil
}

const (
	testCaseAllPartyAccept = iota
	testCaseOnePartyReject
	testCaseOnlyOnePartyAccept
	testCaseAllPartyNull
)

func setJobApprovalStatus(job *kusciaapisv1alpha1.KusciaJob, testCase int) {
	switch testCase {
	case testCaseAllPartyAccept:
		setJobAllPartyAccept(job)
	case testCaseOnePartyReject:
		setJobOnePartyReject(job)
	case testCaseOnlyOnePartyAccept:
		setJobOnlyOnePartyAccept(job)
	case testCaseAllPartyNull:
		setJobAllPartyNil(job)
	}
}

func TestAwaitingApprovalHandler_HandlePhase(t *testing.T) {
	t.Parallel()
	independentJob := makeKusciaJob(KusciaJobForShapeIndependent,
		kusciaapisv1alpha1.KusciaJobScheduleModeBestEffort, 2, nil)

	type fields struct {
		kubeClient   kubernetes.Interface
		kusciaClient versioned.Interface
	}
	type args struct {
		kusciaJob *kusciaapisv1alpha1.KusciaJob
		testCase  int
	}
	tests := []struct {
		name           string
		fields         fields
		args           args
		wantNeedUpdate bool
		wantErr        assert.ErrorAssertionFunc
		wantJobPhase   kusciaapisv1alpha1.KusciaJobPhase
		wantFinalTasks map[string]taskAssertionFunc
	}{
		{
			name: "All party accept should return needUpdate{true} err{nil} phase{pending}",
			fields: fields{
				kubeClient:   kubefake.NewSimpleClientset(),
				kusciaClient: kusciafake.NewSimpleClientset(),
			},
			args: args{
				kusciaJob: independentJob,
				testCase:  testCaseAllPartyAccept,
			},
			wantNeedUpdate: true,
			wantErr:        assert.NoError,
			wantJobPhase:   kusciaapisv1alpha1.KusciaJobPending,
		},
		{
			name: "one party reject should return needUpdate{true} err{nil} phase{approvalReject}",
			fields: fields{
				kubeClient:   kubefake.NewSimpleClientset(),
				kusciaClient: kusciafake.NewSimpleClientset(),
			},
			args: args{
				kusciaJob: independentJob,
				testCase:  testCaseOnePartyReject,
			},
			wantNeedUpdate: true,
			wantErr:        assert.NoError,
			wantJobPhase:   kusciaapisv1alpha1.KusciaJobApprovalReject,
		},
		{
			name: "only one party accept should return needUpdate{false} err{nil} phase{awaitingApproval}",
			fields: fields{
				kubeClient:   kubefake.NewSimpleClientset(),
				kusciaClient: kusciafake.NewSimpleClientset(),
			},
			args: args{
				kusciaJob: independentJob,
				testCase:  testCaseOnlyOnePartyAccept,
			},
			wantNeedUpdate: false,
			wantErr:        assert.NoError,
			wantJobPhase:   kusciaapisv1alpha1.KusciaJobAwaitingApproval,
		},
		{
			name: "all party nil should return needUpdate{false} err{nil} phase{awaitingApproval}",
			fields: fields{
				kubeClient:   kubefake.NewSimpleClientset(),
				kusciaClient: kusciafake.NewSimpleClientset(),
			},
			args: args{
				kusciaJob: independentJob,
				testCase:  testCaseAllPartyNull,
			},
			wantNeedUpdate: false,
			wantErr:        assert.NoError,
			wantJobPhase:   kusciaapisv1alpha1.KusciaJobAwaitingApproval,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kusciaInformerFactory := kusciainformers.NewSharedInformerFactory(tt.fields.kusciaClient, 5*time.Minute)
			kubeInformerFactory := informers.NewSharedInformerFactory(tt.fields.kubeClient, 5*time.Minute)
			nsInformer := kubeInformerFactory.Core().V1().Namespaces()
			domainInformer := kusciaInformerFactory.Kuscia().V1alpha1().Domains()
			aliceNs := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "alice",
				},
			}
			bobNs := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "bob",
				},
			}
			aliceD := &kusciaapisv1alpha1.Domain{
				ObjectMeta: metav1.ObjectMeta{
					Name: "alice",
				},
				Spec: kusciaapisv1alpha1.DomainSpec{},
			}
			bobD := &kusciaapisv1alpha1.Domain{
				ObjectMeta: metav1.ObjectMeta{
					Name: "bob",
				},
				Spec: kusciaapisv1alpha1.DomainSpec{},
			}
			nsInformer.Informer().GetStore().Add(aliceNs)
			nsInformer.Informer().GetStore().Add(bobNs)
			domainInformer.Informer().GetStore().Add(aliceD)
			domainInformer.Informer().GetStore().Add(bobD)
			setJobApprovalStatus(tt.args.kusciaJob, tt.args.testCase)
			deps := &Dependencies{
				KusciaClient:          tt.fields.kusciaClient,
				KusciaTaskLister:      kusciaInformerFactory.Kuscia().V1alpha1().KusciaTasks().Lister(),
				NamespaceLister:       nsInformer.Lister(),
				DomainLister:          domainInformer.Lister(),
				EnableWorkloadApprove: true,
			}
			h := &AwaitingApprovalHandler{
				JobScheduler: NewJobScheduler(deps),
			}
			gotNeedUpdate, err := h.HandlePhase(tt.args.kusciaJob)
			if !tt.wantErr(t, err, fmt.Sprintf("HandlePhase(%v)", tt.args.kusciaJob)) {
				return
			}
			assert.Equalf(t, tt.wantNeedUpdate, gotNeedUpdate, "HandlePhase(%v)", tt.args.kusciaJob)
			assert.Equalf(t, tt.wantJobPhase, tt.args.kusciaJob.Status.Phase, "HandlePhase(%v)", tt.args.kusciaJob)
		})
	}
}
