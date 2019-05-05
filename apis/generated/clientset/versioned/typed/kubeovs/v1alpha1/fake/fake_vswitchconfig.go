/*
Licensed to the Apache Software Foundation (ASF) under one
or more contributor license agreements.  See the NOTICE file
distributed with this work for additional information
regarding copyright ownership.  The ASF licenses this file
to you under the Apache License, Version 2.0 (the
"License"); you may not use this file except in compliance
with the License.  You may obtain a copy of the License at

  http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing,
software distributed under the License is distributed on an
"AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
KIND, either express or implied.  See the License for the
specific language governing permissions and limitations
under the License.
*/

// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	v1alpha1 "github.com/kube-ovs/kube-ovs/apis/kubeovs/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeVSwitchConfigs implements VSwitchConfigInterface
type FakeVSwitchConfigs struct {
	Fake *FakeKubeovsV1alpha1
}

var vswitchconfigsResource = schema.GroupVersionResource{Group: "kubeovs.io", Version: "v1alpha1", Resource: "vswitchconfigs"}

var vswitchconfigsKind = schema.GroupVersionKind{Group: "kubeovs.io", Version: "v1alpha1", Kind: "VSwitchConfig"}

// Get takes name of the vSwitchConfig, and returns the corresponding vSwitchConfig object, and an error if there is any.
func (c *FakeVSwitchConfigs) Get(name string, options v1.GetOptions) (result *v1alpha1.VSwitchConfig, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootGetAction(vswitchconfigsResource, name), &v1alpha1.VSwitchConfig{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.VSwitchConfig), err
}

// List takes label and field selectors, and returns the list of VSwitchConfigs that match those selectors.
func (c *FakeVSwitchConfigs) List(opts v1.ListOptions) (result *v1alpha1.VSwitchConfigList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootListAction(vswitchconfigsResource, vswitchconfigsKind, opts), &v1alpha1.VSwitchConfigList{})
	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1alpha1.VSwitchConfigList{ListMeta: obj.(*v1alpha1.VSwitchConfigList).ListMeta}
	for _, item := range obj.(*v1alpha1.VSwitchConfigList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested vSwitchConfigs.
func (c *FakeVSwitchConfigs) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewRootWatchAction(vswitchconfigsResource, opts))
}

// Create takes the representation of a vSwitchConfig and creates it.  Returns the server's representation of the vSwitchConfig, and an error, if there is any.
func (c *FakeVSwitchConfigs) Create(vSwitchConfig *v1alpha1.VSwitchConfig) (result *v1alpha1.VSwitchConfig, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootCreateAction(vswitchconfigsResource, vSwitchConfig), &v1alpha1.VSwitchConfig{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.VSwitchConfig), err
}

// Update takes the representation of a vSwitchConfig and updates it. Returns the server's representation of the vSwitchConfig, and an error, if there is any.
func (c *FakeVSwitchConfigs) Update(vSwitchConfig *v1alpha1.VSwitchConfig) (result *v1alpha1.VSwitchConfig, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateAction(vswitchconfigsResource, vSwitchConfig), &v1alpha1.VSwitchConfig{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.VSwitchConfig), err
}

// Delete takes name of the vSwitchConfig and deletes it. Returns an error if one occurs.
func (c *FakeVSwitchConfigs) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewRootDeleteAction(vswitchconfigsResource, name), &v1alpha1.VSwitchConfig{})
	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeVSwitchConfigs) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewRootDeleteCollectionAction(vswitchconfigsResource, listOptions)

	_, err := c.Fake.Invokes(action, &v1alpha1.VSwitchConfigList{})
	return err
}

// Patch applies the patch and returns the patched vSwitchConfig.
func (c *FakeVSwitchConfigs) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.VSwitchConfig, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootPatchSubresourceAction(vswitchconfigsResource, name, pt, data, subresources...), &v1alpha1.VSwitchConfig{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.VSwitchConfig), err
}
