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
// Code generated by informer-gen. DO NOT EDIT.

package v1

import (
	"context"
	time "time"

	caelusv1 "github.com/tencent/caelus/pkg/apis/caelus/v1"
	versioned "github.com/tencent/caelus/pkg/generated/clientset/versioned"
	internalinterfaces "github.com/tencent/caelus/pkg/generated/informers/externalversions/internalinterfaces"
	v1 "github.com/tencent/caelus/pkg/generated/listers/caelus/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	watch "k8s.io/apimachinery/pkg/watch"
	cache "k8s.io/client-go/tools/cache"
)

// RuleCheckInformer provides access to a shared informer and lister for
// RuleChecks.
type RuleCheckInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() v1.RuleCheckLister
}

type ruleCheckInformer struct {
	factory          internalinterfaces.SharedInformerFactory
	tweakListOptions internalinterfaces.TweakListOptionsFunc
	namespace        string
}

// NewRuleCheckInformer constructs a new informer for RuleCheck type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewRuleCheckInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers) cache.SharedIndexInformer {
	return NewFilteredRuleCheckInformer(client, namespace, resyncPeriod, indexers, nil)
}

// NewFilteredRuleCheckInformer constructs a new informer for RuleCheck type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewFilteredRuleCheckInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc) cache.SharedIndexInformer {
	return cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.CaelusV1().RuleChecks(namespace).List(context.TODO(), options)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.CaelusV1().RuleChecks(namespace).Watch(context.TODO(), options)
			},
		},
		&caelusv1.RuleCheck{},
		resyncPeriod,
		indexers,
	)
}

func (f *ruleCheckInformer) defaultInformer(client versioned.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
	return NewFilteredRuleCheckInformer(client, f.namespace, resyncPeriod, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, f.tweakListOptions)
}

func (f *ruleCheckInformer) Informer() cache.SharedIndexInformer {
	return f.factory.InformerFor(&caelusv1.RuleCheck{}, f.defaultInformer)
}

func (f *ruleCheckInformer) Lister() v1.RuleCheckLister {
	return v1.NewRuleCheckLister(f.Informer().GetIndexer())
}
