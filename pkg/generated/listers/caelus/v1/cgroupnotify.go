/*
Copyright The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Code generated by lister-gen. DO NOT EDIT.

package v1

import (
	v1 "github.com/tencent/caelus/pkg/apis/caelus/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

// CgroupNotifyLister helps list CgroupNotifies.
// All objects returned here must be treated as read-only.
type CgroupNotifyLister interface {
	// List lists all CgroupNotifies in the indexer.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1.CgroupNotify, err error)
	// CgroupNotifies returns an object that can list and get CgroupNotifies.
	CgroupNotifies(namespace string) CgroupNotifyNamespaceLister
	CgroupNotifyListerExpansion
}

// cgroupNotifyLister implements the CgroupNotifyLister interface.
type cgroupNotifyLister struct {
	indexer cache.Indexer
}

// NewCgroupNotifyLister returns a new CgroupNotifyLister.
func NewCgroupNotifyLister(indexer cache.Indexer) CgroupNotifyLister {
	return &cgroupNotifyLister{indexer: indexer}
}

// List lists all CgroupNotifies in the indexer.
func (s *cgroupNotifyLister) List(selector labels.Selector) (ret []*v1.CgroupNotify, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v1.CgroupNotify))
	})
	return ret, err
}

// CgroupNotifies returns an object that can list and get CgroupNotifies.
func (s *cgroupNotifyLister) CgroupNotifies(namespace string) CgroupNotifyNamespaceLister {
	return cgroupNotifyNamespaceLister{indexer: s.indexer, namespace: namespace}
}

// CgroupNotifyNamespaceLister helps list and get CgroupNotifies.
// All objects returned here must be treated as read-only.
type CgroupNotifyNamespaceLister interface {
	// List lists all CgroupNotifies in the indexer for a given namespace.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1.CgroupNotify, err error)
	// Get retrieves the CgroupNotify from the indexer for a given namespace and name.
	// Objects returned here must be treated as read-only.
	Get(name string) (*v1.CgroupNotify, error)
	CgroupNotifyNamespaceListerExpansion
}

// cgroupNotifyNamespaceLister implements the CgroupNotifyNamespaceLister
// interface.
type cgroupNotifyNamespaceLister struct {
	indexer   cache.Indexer
	namespace string
}

// List lists all CgroupNotifies in the indexer for a given namespace.
func (s cgroupNotifyNamespaceLister) List(selector labels.Selector) (ret []*v1.CgroupNotify, err error) {
	err = cache.ListAllByNamespace(s.indexer, s.namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*v1.CgroupNotify))
	})
	return ret, err
}

// Get retrieves the CgroupNotify from the indexer for a given namespace and name.
func (s cgroupNotifyNamespaceLister) Get(name string) (*v1.CgroupNotify, error) {
	obj, exists, err := s.indexer.GetByKey(s.namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(v1.Resource("cgroupnotify"), name)
	}
	return obj.(*v1.CgroupNotify), nil
}