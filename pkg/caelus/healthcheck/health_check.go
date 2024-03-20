/*
 * Copyright (c) 2021 THL A29 Limited, a Tencent company.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 *
 * You may obtain a copy of the License at http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package health

import (
	"reflect"
	"sort"

	v1 "github.com/tencent/caelus/pkg/apis/caelus/v1"
	cgroupCrd "github.com/tencent/caelus/pkg/apis/cgroupnotifycrd/v1"
	notify "github.com/tencent/caelus/pkg/caelus/healthcheck/cgroupnotify"
	"github.com/tencent/caelus/pkg/caelus/healthcheck/conflict"
	"github.com/tencent/caelus/pkg/caelus/healthcheck/rulecheck"
	"github.com/tencent/caelus/pkg/caelus/qos"
	"github.com/tencent/caelus/pkg/caelus/resource"
	"github.com/tencent/caelus/pkg/caelus/statestore"
	"github.com/tencent/caelus/pkg/caelus/types"
	"github.com/tencent/caelus/pkg/caelus/util"
	cgroupInformer "github.com/tencent/caelus/pkg/cgroupClient/informers/externalversions/cgroupnotifycrd/v1"
	caelusv1 "github.com/tencent/caelus/pkg/generated/informers/externalversions/caelus/v1"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels2 "k8s.io/apimachinery/pkg/labels"
	informerv1 "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"
)

const (
	checkConfigFile = "/etc/caelus/rules.json"
	ruleCheck       = "RuleCheck"
	cgroupNotify    = "CgroupNotify"
)

// Manager is the interface for handling health check
type Manager interface {
	Name() string
	Run(stop <-chan struct{})
}

// manager detect container health based on metrics
type manager struct {
	config      *types.HealthCheckConfig
	ruleChecker *rulecheck.Manager
	// support linux kernel PSI, now just memory event
	cgroupNotifier         notify.ResourceNotify
	stStore                statestore.StateStore
	resource               resource.Interface
	qosManager             qos.Manager
	conflictMn             conflict.Manager
	podInformer            cache.SharedIndexInformer
	nodeInformer           informerv1.NodeInformer
	ruleCheckInformer      caelusv1.RuleCheckInformer
	cgroupInformer         cgroupInformer.CgroupNotifyCrdInformer
	workqueue              workqueue.RateLimitingInterface
	configHash             string
	globalStopCh           <-chan struct{}
	ruleCheckAvailableFunc func(ruleCheck *types.RuleCheckConfig)
}

// NewHealthManager create a new health check manager
func NewHealthManager(stStore statestore.StateStore,
	resource resource.Interface, qosManager qos.Manager, conflictMn conflict.Manager,
	podInformer cache.SharedIndexInformer, nodeInformer informerv1.NodeInformer,
	ruleCheckInformer caelusv1.RuleCheckInformer, cgroupInformer cgroupInformer.CgroupNotifyCrdInformer,
	resourceType *types.Resource, ruleCheckAvailableFunc func(ruleCheck *types.RuleCheckConfig)) Manager {

	config := &types.HealthCheckConfig{
		RuleCheck: types.RuleCheck{
			ContainerRules: []*types.RuleCheckConfig{},
			NodeRules:      []*types.RuleCheckConfig{},
			AppRules:       []*types.RuleCheckConfig{},
		},
		PredictReserved: resourceType,
	}
	hm := &manager{
		config:                 config,
		ruleChecker:            rulecheck.NewManager(config.RuleCheck, stStore, resource, qosManager, conflictMn, podInformer, config.PredictReserved),
		cgroupNotifier:         notify.NewNotifyManager(&config.CgroupNotify, resource),
		stStore:                stStore,
		resource:               resource,
		qosManager:             qosManager,
		conflictMn:             conflictMn,
		podInformer:            podInformer,
		nodeInformer:           nodeInformer,
		workqueue:              workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "health-check-queue"),
		ruleCheckInformer:      ruleCheckInformer,
		cgroupInformer:         cgroupInformer,
		ruleCheckAvailableFunc: ruleCheckAvailableFunc,
	}

	_, err := hm.ruleCheckInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			hm.eventFunc([]interface{}{obj}, ruleCheck)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			hm.eventFunc([]interface{}{oldObj, newObj}, ruleCheck)
		},
		DeleteFunc: func(obj interface{}) {
			hm.eventFunc([]interface{}{obj}, ruleCheck)
		},
	})
	if err != nil {
		klog.Error("Failed to add RuleCheck event func", err)
		return nil
	}

	_, err = hm.nodeInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(oldObj, newObj interface{}) {
			if isLocalNodeEventAndLabelChanged(oldObj, newObj) {
				hm.workqueue.Add(cgroupNotify)
				hm.workqueue.Add(ruleCheck)
			}
		},
	})
	if err != nil {
		klog.Error("Failed to add Node event func", err)
		return nil
	}

	_, err = hm.cgroupInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			hm.eventFunc([]interface{}{obj}, cgroupNotify)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			hm.eventFunc([]interface{}{oldObj, newObj}, cgroupNotify)
		},
		DeleteFunc: func(obj interface{}) {
			hm.eventFunc([]interface{}{obj}, cgroupNotify)
		},
	})
	if err != nil {
		klog.Error("Failed to add CgroupNotify event func", err)
		return nil
	}

	return hm
}

// Name returns the module name
func (h *manager) Name() string {
	return "ModuleHealthCheck"
}

// filter the event affects local node and add cr type to the queue
func (h *manager) eventFunc(crs []interface{}, crd string) {
	if h.isAffectingLocalNode(crs) {
		h.workqueue.Add(crd)
	}
}

// Run start checking health
func (h *manager) Run(stop <-chan struct{}) {
	// the function is running in the informer evnet func
	h.globalStopCh = stop

	// init the ruleCheck and cgroupNotify for new node
	h.workqueue.Add(cgroupNotify)
	h.workqueue.Add(ruleCheck)

	go h.runWorker()
}

func (h *manager) runWorker() {
	for h.processNextWorkItem() {
	}
}

func (h *manager) processNextWorkItem() bool {
	select {
	case <-h.globalStopCh:
		h.workqueue.ShutDown()
		return false
	default:
	}
	obj, shutdown := h.workqueue.Get()

	if shutdown {
		return false
	}

	key, ok := obj.(string)
	if !ok {
		h.workqueue.Forget(obj)
		h.workqueue.Done(obj)
		klog.Errorf("Health check queue item expects string but got %#v", obj)
		return true
	}

	if err := h.syncHandler(key); err != nil {
		h.workqueue.Add(key)
	}

	h.workqueue.Done(obj)
	return true
}

func (h *manager) syncHandler(key string) error {
	klog.Infof("---- Start syncHandler %v: %v", key, *h.config)
	if key == ruleCheck {
		err := h.updateRuleCheckConfig()
		if err != nil {
			return err
		}
		h.reRunRuleCheck()
	} else if key == cgroupNotify {
		//err := h.updateCgroupConfig()
		//if err != nil{
		//	return err
		//}
		//h.reRunCgroupNotifier()
	}

	klog.Infof("---- After syncHandler %v: %v", key, *h.config)
	return nil
}

// reRunRuleCheck update and rerun the ruleChecker by the config
func (h *manager) reRunRuleCheck() {
	if h.ruleChecker != nil {
		h.ruleChecker.Stop()
	}

	h.ruleChecker = rulecheck.NewManager(h.config.RuleCheck, h.stStore, h.resource, h.qosManager, h.conflictMn, h.podInformer, h.config.PredictReserved)

	if h.ruleChecker != nil {
		go h.ruleChecker.Run(h.globalStopCh)
	}
}

// reRunCgroupNotifier update and rerun the cgroupNotifier by the config
func (h *manager) reRunCgroupNotifier() {
	if h.cgroupNotifier != nil {
		h.cgroupNotifier.Stop()
	}

	h.cgroupNotifier = notify.NewNotifyManager(&h.config.CgroupNotify, h.resource)

	if h.cgroupNotifier != nil {
		go h.cgroupNotifier.Run(h.globalStopCh)
	}
}

func (h *manager) OnAddCgropNotify(obj interface{}) {
	h.updateCgroupConfig(obj, false)
	h.reRunCgroupNotifier()
}

func (h *manager) OnUpdateCgropNotify(oldObj, newObj interface{}) {
	h.updateCgroupConfig(newObj, false)
	h.reRunCgroupNotifier()
}

func (h *manager) OnDeleteCgropNotify(obj interface{}) {
	h.updateCgroupConfig(obj, true)
	h.reRunCgroupNotifier()
}

func (h *manager) CgroupCrddeepCopy(cgroupCrd *cgroupCrd.CgroupNotifyCrd) {
	pressures := make([]types.MemoryPressureNotifyConfig, 0)
	for _, p := range cgroupCrd.Spec.MemoryCgroup.Pressures {
		newP := types.MemoryPressureNotifyConfig{}
		newP.Cgroups = p.Cgroups
		newP.Count = p.Count
		newP.Duration = p.Duration
		newP.PressureLevel = p.PressureLevel
		pressures = append(pressures, newP)
	}
	usages := make([]types.MemoryUsageNotifyConfig, 0)
	for _, u := range cgroupCrd.Spec.MemoryCgroup.Usages {
		newU := types.MemoryUsageNotifyConfig{}
		newU.Cgroups = u.Cgroups
		newU.Duration = u.Duration
		newU.MarginMb = u.MarginMb
		usages = append(usages, newU)
	}
	h.config.CgroupNotify.MemoryCgroup.Pressures = pressures
	h.config.CgroupNotify.MemoryCgroup.Usages = usages
	h.config.CgroupNotify.Labels = cgroupCrd.Spec.NodeSelector.MatchLabels
}

func (h *manager) updateCgroupConfig(obj interface{}, isDelete bool) {
	object := obj.(metav1.Object)
	ownerRef := metav1.GetControllerOf(object)
	cgroupCrd, err := h.cgroupInformer.Lister().CgroupNotifyCrds(object.GetNamespace()).Get(ownerRef.Name)
	if err != nil {
		klog.Error("get cgroupNotifyCrd failed")
		return
	}
	if isDelete {
		h.config.CgroupNotify = types.NotifyConfig{}
	} else {
		h.CgroupCrddeepCopy(cgroupCrd)
	}
}

// update local RuleCheck
func (h *manager) updateRuleCheckConfig() error {
	ruleChecks, err := h.ruleCheckInformer.Lister().RuleChecks(types.CaelusNamespace).List(labels2.SelectorFromSet(nil))
	if err != nil {
		return err
	}
	sort.Slice(ruleChecks, func(i, j int) bool {
		if *(ruleChecks[i].Spec.Priority) == *(ruleChecks[j].Spec.Priority) {
			return ruleChecks[i].CreationTimestamp.Time.After(ruleChecks[i].CreationTimestamp.Time)
		} else {
			return *(ruleChecks[i].Spec.Priority) > *(ruleChecks[j].Spec.Priority)
		}
	})

	m := map[string]struct{}{}
	appRuleChecks := []*types.RuleCheckConfig{}
	nodeRuleChecks := []*types.RuleCheckConfig{}
	containerRuleChecks := []*types.RuleCheckConfig{}
	for _, k8sRuleCheck := range ruleChecks {
		if ok, err := h.isLabelMatchedLocalNode(k8sRuleCheck.Spec.NodeSelector.MatchLabels); err != nil || !ok {
			continue
		}
		name := string(k8sRuleCheck.Spec.Type) + ":" + k8sRuleCheck.Spec.Name
		if _, ok := m[name]; ok {
			continue
		}
		m[name] = struct{}{}
		ruleCheck := &types.RuleCheckConfig{}
		h.convertK8sRuleCheck(k8sRuleCheck, ruleCheck)
		switch k8sRuleCheck.Spec.Type {
		case v1.AppType:
			appRuleChecks = append(appRuleChecks, ruleCheck)
		case v1.NodeType:
			nodeRuleChecks = append(nodeRuleChecks, ruleCheck)
		case v1.ContainerType:
			containerRuleChecks = append(containerRuleChecks, ruleCheck)
		}
	}
	h.config.RuleCheck.AppRules = appRuleChecks
	h.config.RuleCheck.NodeRules = nodeRuleChecks
	h.config.RuleCheck.ContainerRules = containerRuleChecks
	return nil
}

// convert v1.RuleCheck struct to types.RuleCheckConfig struct
func (h *manager) convertK8sRuleCheck(k8sRuleCheck *v1.RuleCheck, ruleCheck *types.RuleCheckConfig) {
	convertRules := func(k8sRules []*v1.DetectActionRules) []*types.DetectActionConfig {
		rules := make([]*types.DetectActionConfig, 0, len(k8sRuleCheck.Spec.Rules))
		for _, k8sRule := range k8sRules {
			rule := &types.DetectActionConfig{
				Detects: make([]*types.DetectConfig, 0, len(k8sRule.Detects)),
				Actions: make([]*types.ActionConfig, 0, len(k8sRule.Actions)),
			}
			for _, detect := range k8sRule.Detects {
				rule.Detects = append(rule.Detects, &types.DetectConfig{
					Name:    detect.Name,
					ArgsStr: []byte(detect.Args),
					Args:    nil,
				})
			}
			for _, action := range k8sRule.Actions {
				rule.Actions = append(rule.Actions, &types.ActionConfig{
					Name:    action.Name,
					ArgsStr: []byte(action.Args),
					Args:    nil,
				})
			}
			rules = append(rules, rule)
		}
		return rules
	}

	ruleCheck.Rules = convertRules(k8sRuleCheck.Spec.Rules)
	ruleCheck.RecoverRules = convertRules(k8sRuleCheck.Spec.RecoverRules)

	ruleCheck.Name = k8sRuleCheck.Name
	ruleCheck.Metrics = k8sRuleCheck.Spec.Metrics
	ruleCheck.CheckInterval = *k8sRuleCheck.Spec.CheckInterval

	ruleCheck.RecoverInterval = *k8sRuleCheck.Spec.RecoverInterval
	ruleCheck.HandleInterval = *k8sRuleCheck.Spec.HandleInterval
	ruleCheck.NodeSelector = k8sRuleCheck.Spec.NodeSelector
	h.ruleCheckAvailableFunc(ruleCheck)
}

// determine whether this cr event impacts this node
func (h *manager) isAffectingLocalNode(objs []interface{}) bool {
	for _, obj := range objs {
		if cgroupCr, ok := obj.(*cgroupCrd.CgroupNotifyCrd); ok {
			if matched, err := h.isLabelMatchedLocalNode(cgroupCr.Spec.NodeSelector); matched || err != nil {
				return true
			}
		} else if ruleCheckCr, ok := obj.(*v1.RuleCheck); ok {
			if matched, err := h.isLabelMatchedLocalNode(ruleCheckCr.Spec.NodeSelector); matched || err != nil {
				return true
			}
		}
	}
	return false
}

// isLabelMatched determines whether the labels match this node
func (h *manager) isLabelMatchedLocalNode(labels map[string]string) (bool, error) {
	if len(labels) == 0 {
		return true, nil
	}

	node, err := h.nodeInformer.Lister().Get(util.NodeName())
	if err != nil {
		return false, err
	}
	for key, val := range labels {
		if val != node.Labels[key] {
			return false, nil
		}
	}
	return true, nil
}

// determine whether this node event happened in local node and label changed
func isLocalNodeEventAndLabelChanged(oldObj, newObj interface{}) bool {
	oldCr, ok := oldObj.(*corev1.Node)
	if !ok {
		return false
	}
	newCr, ok := newObj.(*corev1.Node)
	if !ok {
		return false
	}

	if newCr.Name != util.NodeName() {
		return false
	}

	return !reflect.DeepEqual(oldCr.Labels, newCr.Labels)
}
