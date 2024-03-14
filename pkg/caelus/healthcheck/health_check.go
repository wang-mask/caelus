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
	"crypto/md5"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"

	"github.com/fsnotify/fsnotify"
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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

const (
	checkConfigFile = "/etc/caelus/rules.json"
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
	cgroupNotifier notify.ResourceNotify
	stStore        statestore.StateStore
	resource       resource.Interface
	qosManager     qos.Manager
	conflictMn     conflict.Manager
	podInformer    cache.SharedIndexInformer
	configHash     string
	globalStopCh   <-chan struct{}
	// xxxInformer1
	cgroupInformer cgroupInformer.CgroupNotifyCrdInformer
	// xxxInformer2
	k8sClient clientset.Interface
}

// NewHealthManager create a new health check manager
func NewHealthManager(stStore statestore.StateStore,
	resource resource.Interface, qosManager qos.Manager, conflictMn conflict.Manager,
	podInformer cache.SharedIndexInformer, cgroupInformer cgroupInformer.CgroupNotifyCrdInformer,
	k8sClient clientset.Interface) Manager {

	// TODO add the informer enent func

	hm := &manager{
		stStore:     stStore,
		resource:    resource,
		qosManager:  qosManager,
		conflictMn:  conflictMn,
		podInformer: podInformer,
		k8sClient:   k8sClient,
		// add informer
		cgroupInformer: cgroupInformer,
	}

	hm.cgroupInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    hm.OnAddCgropNotify,
		UpdateFunc: hm.OnUpdateCgropNotify,
		DeleteFunc: hm.OnDeleteCgropNotify,
	})

	return hm
}

// Name returns the module name
func (h *manager) Name() string {
	return "ModuleHealthCheck"
}

// reload rule check config dynamically without restarting the agent
func (h *manager) reload() {
	reload, hash, config := h.checkNeedReload(checkConfigFile)
	if !reload {
		return
	}
	h.configHash = hash
	h.ruleChecker.Stop()
	h.cgroupNotifier.Stop()
	h.config = config
	h.ruleChecker = rulecheck.NewManager(config.RuleCheck, h.stStore, h.resource, h.qosManager, h.conflictMn,
		h.podInformer, config.PredictReserved)
	h.cgroupNotifier = notify.NewNotifyManager(&config.CgroupNotify, h.resource)
	go h.ruleChecker.Run(h.globalStopCh)
	go h.cgroupNotifier.Run(h.globalStopCh)
}

// checkNeedReload checks if the config file is changed
func (h *manager) checkNeedReload(configFile string) (bool, string, *types.HealthCheckConfig) {
	hash, err := hashFile(configFile)
	if err != nil {
		klog.Errorf("failed hash config file: %v", err)
		return false, "", nil
	}
	if hash == h.configHash {
		return false, "", nil
	}
	config, err := h.configUpdateFunc(configFile)
	if err != nil {
		klog.Fatalf("failed init health check config: %v", err)
	}
	if len(config.RuleNodes) != 0 {
		found := false
		for _, no := range config.RuleNodes {
			if no == util.NodeIP() {
				found = true
				break
			}
		}
		if !found {
			return false, "", nil
		}
	}

	return true, hash, config
}

// Run start checking health
func (h *manager) Run(stop <-chan struct{}) {
	h.globalStopCh = stop
	<-stop
}

// configWatcher support reload rule check config dynamically, no need to restart the agent
func (h *manager) configWatcher(stop <-chan struct{}) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		klog.Fatalf("failed init fsnotify watcher: %v", err)
	}
	defer w.Close()
	err = w.Add(filepath.Dir(checkConfigFile))
	if err != nil {
		klog.Fatalf("failed add dir watcher(%s): %v", filepath.Dir(checkConfigFile), err)
	}
	for {
		select {
		case <-w.Events:
			h.reload()
		case err := <-w.Errors:
			klog.Errorf("fsnotify error: %v", err)
		case <-stop:
			return
		}
	}
}

// hashFile generate hash code for the file
func hashFile(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := md5.New()
	if _, err = io.Copy(hash, file); err != nil {
		return "", err
	}
	h := hash.Sum(nil)[:16]
	hs := hex.EncodeToString(h)
	return hs, nil
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

func (h *manager) updateRulecheck() {

}

func (h *manager) updateCgroupNotifier() {

}

func (h *manager) HasNode(cgroupCrd *cgroupCrd.CgroupNotifyCrd) (bool, error) {
	nodeSelector := cgroupCrd.Spec.NodeSelector
	nodes, err := h.k8sClient.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{
		LabelSelector: metav1.FormatLabelSelector(&nodeSelector),
	})
	if err != nil {
		return false, err
	}

	found := false
	for _, n := range nodes.Items {
		if n.Name == util.NodeName() {
			found = true
			break
		}
	}
	return found, nil
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
}

func (h *manager) updateCgroupConfig(obj interface{}, isDelete bool) {
	object := obj.(metav1.Object)
	ownerRef := metav1.GetControllerOf(object)
	cgroupCrd, err := h.cgroupInformer.Lister().CgroupNotifyCrds(object.GetNamespace()).Get(ownerRef.Name)
	if err != nil {
		klog.Error("get cgroupNotifyCrd failed")
		return
	}
	found, err := h.HasNode(cgroupCrd)
	if err != nil {
		klog.Error("get Nodes failed: %v", err)
		return
	}
	if !found {
		return
	}
	if isDelete {
		h.config.CgroupNotify = types.NotifyConfig{}
	} else {
		h.CgroupCrddeepCopy(cgroupCrd)
	}
}
