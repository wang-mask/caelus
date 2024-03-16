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
	"context"
	"fmt"
	"sort"
	"strings"

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
	caelusclient "github.com/tencent/caelus/pkg/generated/clientset/versioned"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"
)

const (
	checkConfigFile = "/etc/caelus/rules.json"
	ruleCheck = "RuleCheck"
	cgroupNotify = "CgroupNotify"
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
	cgroupNotifier    notify.ResourceNotify
	stStore           statestore.StateStore
	resource          resource.Interface
	qosManager        qos.Manager
	conflictMn        conflict.Manager
	podInformer       cache.SharedIndexInformer
	nodeInformer      cache.SharedIndexInformer
	workqueue         workqueue.RateLimitingInterface
	configHash        string
	globalStopCh      <-chan struct{}
	ruleCheckInformer cache.SharedIndexInformer
	caelusClient      caelusclient.Clientset
	cgroupInformer    cgroupInformer.CgroupNotifyCrdInformer
	k8sClient         clientset.Interface
}

// NewHealthManager create a new health check manager
func NewHealthManager(stStore statestore.StateStore,
	resource resource.Interface, qosManager qos.Manager, conflictMn conflict.Manager,
	podInformer cache.SharedIndexInformer, nodeInformer cache.SharedIndexInformer, ruleCheckInformer cache.SharedIndexInformer, cgroupInformer cgroupInformer.CgroupNotifyCrdInformer,
	k8sClient clientset.Interface) Manager {

	// TODO add the informer enent func

	hm := &manager{
		stStore:      stStore,
		resource:     resource,
		qosManager:   qosManager,
		conflictMn:   conflictMn,
		podInformer:  podInformer,
		nodeInformer: nodeInformer,
		workqueue:    workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "health-check-queue"),
		k8sClient:    k8sClient,
		// add informer
		ruleCheckInformer: ruleCheckInformer,
		cgroupInformer:    cgroupInformer,
	}

	_, err := hm.ruleCheckInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) {
			hm.eventFunc([]string{ruleCheck})
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			hm.eventFunc([]string{ruleCheck})
		},
		DeleteFunc: func(obj interface{}) {
			hm.eventFunc([]string{ruleCheck})
		},
	})
	if err != nil {
		return nil
	}
	hm.nodeInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) {
			hm.eventFunc([]string{ruleCheck, cgroupNotify})
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			hm.eventFunc([]string{ruleCheck, cgroupNotify})
		},
		DeleteFunc: func(obj interface{}) {
			hm.eventFunc([]string{ruleCheck, cgroupNotify})
		},
	})

	hm.cgroupInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) {
			hm.eventFunc([]string{cgroupNotify})
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			hm.eventFunc([]string{cgroupNotify})
		},
		DeleteFunc: func(obj interface{}) {
			hm.eventFunc([]string{cgroupNotify})
		},
	})

	return hm
}

// Name returns the module name
func (h *manager) Name() string {
	return "ModuleHealthCheck"
}

func(h *manager) eventFunc(crds string[])func(obj interface{}){
	return func(obj interface{}){
		for crd := range crds{
			h.workqueue.Add(crd)
		}
	}
}

//// reload rule check config dynamically without restarting the agent
//func (h *manager) reload() {
//	reload, hash, config := h.checkNeedReload(checkConfigFile)
//	if !reload {
//		return
//	}
//	h.configHash = hash
//	h.ruleChecker.Stop()
//	h.cgroupNotifier.Stop()
//	h.config = config
//	h.ruleChecker = rulecheck.NewManager(config.RuleCheck, h.stStore, h.resource, h.qosManager, h.conflictMn,
//		h.podInformer, config.PredictReserved)
//	h.cgroupNotifier = notify.NewNotifyManager(&config.CgroupNotify, h.resource)
//	go h.ruleChecker.Run(h.globalStopCh)
//	go h.cgroupNotifier.Run(h.globalStopCh)
//}

//// checkNeedReload checks if the config file is changed
//func (h *manager) checkNeedReload(configFile string) (bool, string, *types.HealthCheckConfig) {
//	hash, err := hashFile(configFile)
//	if err != nil {
//		klog.Errorf("failed hash config file: %v", err)
//		return false, "", nil
//	}
//	if hash == h.configHash {
//		return false, "", nil
//	}
//	config, err := h.configUpdateFunc(configFile)
//	if err != nil {
//		klog.Fatalf("failed init health check config: %v", err)
//	}
//	if len(config.RuleNodes) != 0 {
//		found := false
//		for _, no := range config.RuleNodes {
//			if no == util.NodeIP() {
//				found = true
//				break
//			}
//		}
//		if !found {
//			return false, "", nil
//		}
//	}
//
//	return true, hash, config
//}

// Run start checking health
func (h *manager) Run(stop <-chan struct{}) {
	// the function is running in the informer evnet func
	h.globalStopCh = stop
	go h.runWorker(stop)
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
		h.workqueue.AddRateLimited(key)
		return true
	}

	h.workqueue.Done(obj)
	return true
}

func (h *manager) syncHandler(key string) error {

	return nil
}

//// configWatcher support reload rule check config dynamically, no need to restart the agent
//func (h *manager) configWatcher(stop <-chan struct{}) {
//	w, err := fsnotify.NewWatcher()
//	if err != nil {
//		klog.Fatalf("failed init fsnotify watcher: %v", err)
//	}
//	defer w.Close()
//	err = w.Add(filepath.Dir(checkConfigFile))
//	if err != nil {
//		klog.Fatalf("failed add dir watcher(%s): %v", filepath.Dir(checkConfigFile), err)
//	}
//	for {
//		select {
//		case <-w.Events:
//			h.reload()
//		case err := <-w.Errors:
//			klog.Errorf("fsnotify error: %v", err)
//		case <-stop:
//			return
//		}
//	}
//}
//
//// hashFile generate hash code for the file
//func hashFile(filePath string) (string, error) {
//	file, err := os.Open(filePath)
//	if err != nil {
//		return "", err
//	}
//	defer file.Close()
//	hash := md5.New()
//	if _, err = io.Copy(hash, file); err != nil {
//		return "", err
//	}
//	h := hash.Sum(nil)[:16]
//	hs := hex.EncodeToString(h)
//	return hs, nil
//}

// updateRuleCheck update the ruleChecker by the config
func (h *manager) updateRuleCheck() error {
	matched, err := h.isLabelMatched(h.config.CgroupNotify.Labels)
	if err != nil {
		return err
	}

	if matched {
		if h.ruleChecker == nil {
			h.ruleChecker = rulecheck.NewManager(h.config.RuleCheck, h.stStore, h.resource, h.qosManager, h.conflictMn, h.podInformer, h.config.PredictReserved)
			h.ruleChecker.Run(h.globalStopCh)
		} else {
			h.ruleChecker.UpdateManager(h.config.RuleCheck, h.stStore, h.podInformer, h.config.PredictReserved)
		}
	} else {
		if h.ruleChecker != nil {
			h.ruleChecker.Stop()
			h.ruleChecker = nil
		}
	}
	return nil
}

// updateCgroupNotifier update the cgroupNotifier by the config
func (h *manager) updateCgroupNotifier() error {
	matched, err := h.isLabelMatched(h.config.CgroupNotify.Labels)
	if err != nil {
		return err
	}

	if matched {
		if h.cgroupNotifier != nil {
			// in case the cgroupNotifier is not enabled in last update
			h.cgroupNotifier.Stop()
		}
		h.cgroupNotifier = notify.NewNotifyManager(&h.config.CgroupNotify, h.resource)
		go h.cgroupNotifier.Run(h.globalStopCh)
	} else {
		if h.cgroupNotifier != nil {
			// if the labels does not match this node, stop the cgroupNotifier
			h.cgroupNotifier.Stop()
			h.cgroupNotifier = nil
		}
	}
	return nil
}

// isLabelMatched determines whether the labels match this node
func (h *manager) isLabelMatched(labels map[string]string) (bool, error) {
	if len(labels) == 0 {
		return true, nil
	}
	selectors := make([]string, len(labels))
	index := 0
	for key, val := range labels {
		selectors[index] = fmt.Sprintf("%s=%s", key, val)
		index += 1
	}
	nodes, err := h.k8sClient.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{LabelSelector: strings.Join(selectors, ",")})
	if err != nil {
		return false, err
	}
	for _, node := range nodes.Items {
		if node.Name == util.NodeName() {
			return true, nil
		}
	}
	return false, nil
}

func (h *manager) OnAddCgropNotify(obj interface{}) {
	h.updateCgroupConfig(obj, false)
	h.updateCgroupNotifier()
}

func (h *manager) OnUpdateCgropNotify(oldObj, newObj interface{}) {
	h.updateCgroupConfig(newObj, false)
	h.updateCgroupNotifier()
}

func (h *manager) OnDeleteCgropNotify(obj interface{}) {
	h.updateCgroupConfig(obj, true)
	h.updateCgroupNotifier()
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
	ctx := context.Background()
	list, err := h.caelusClient.CaelusV1().RuleChecks("caelus-system").List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}
	ruleChecks := list.Items
	sort.Slice(ruleChecks, func(i, j int) bool {
		return ruleChecks[i].CreationTimestamp.Time.After(ruleChecks[i].CreationTimestamp.Time)
	})

	m := map[string]struct{}{}
	appRuleChecks := []*types.RuleCheckConfig{}
	nodeRuleChecks := []*types.RuleCheckConfig{}
	containerRuleChecks := []*types.RuleCheckConfig{}
	for _, k8sRuleCheck := range ruleChecks {
		name := string(k8sRuleCheck.Spec.Type) + ":" + k8sRuleCheck.Spec.Name
		if _, ok := m[name]; ok {
			continue
		}
		ruleCheck := &types.RuleCheckConfig{}
		convertK8sRuleCheck(&k8sRuleCheck, ruleCheck)
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
func convertK8sRuleCheck(k8sRuleCheck *v1.RuleCheck, ruleCheck *types.RuleCheckConfig) {
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
					ArgsStr: detect.Args,
					Args:    nil,
				})
			}
			for _, action := range k8sRule.Actions {
				rule.Actions = append(rule.Actions, &types.ActionConfig{
					Name:    action.Name,
					ArgsStr: action.Args,
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
}
