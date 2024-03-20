//go:build !ignore_autogenerated
// +build !ignore_autogenerated

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
// Code generated by deepcopy-gen. DO NOT EDIT.

package v1

import (
	times "github.com/tencent/caelus/pkg/util/times"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DetectAction) DeepCopyInto(out *DetectAction) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DetectAction.
func (in *DetectAction) DeepCopy() *DetectAction {
	if in == nil {
		return nil
	}
	out := new(DetectAction)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DetectActionRules) DeepCopyInto(out *DetectActionRules) {
	*out = *in
	if in.Detects != nil {
		in, out := &in.Detects, &out.Detects
		*out = make([]*DetectAction, len(*in))
		for i := range *in {
			if (*in)[i] != nil {
				in, out := &(*in)[i], &(*out)[i]
				*out = new(DetectAction)
				**out = **in
			}
		}
	}
	if in.Actions != nil {
		in, out := &in.Actions, &out.Actions
		*out = make([]*DetectAction, len(*in))
		for i := range *in {
			if (*in)[i] != nil {
				in, out := &(*in)[i], &(*out)[i]
				*out = new(DetectAction)
				**out = **in
			}
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DetectActionRules.
func (in *DetectActionRules) DeepCopy() *DetectActionRules {
	if in == nil {
		return nil
	}
	out := new(DetectActionRules)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *RuleCheck) DeepCopyInto(out *RuleCheck) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	out.Status = in.Status
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new RuleCheck.
func (in *RuleCheck) DeepCopy() *RuleCheck {
	if in == nil {
		return nil
	}
	out := new(RuleCheck)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *RuleCheck) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *RuleCheckList) DeepCopyInto(out *RuleCheckList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]RuleCheck, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new RuleCheckList.
func (in *RuleCheckList) DeepCopy() *RuleCheckList {
	if in == nil {
		return nil
	}
	out := new(RuleCheckList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *RuleCheckList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *RuleCheckSpec) DeepCopyInto(out *RuleCheckSpec) {
	*out = *in
	if in.NodeSelector != nil {
		in, out := &in.NodeSelector, &out.NodeSelector
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.Priority != nil {
		in, out := &in.Priority, &out.Priority
		*out = new(int32)
		**out = **in
	}
	if in.Metrics != nil {
		in, out := &in.Metrics, &out.Metrics
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.CheckInterval != nil {
		in, out := &in.CheckInterval, &out.CheckInterval
		*out = new(times.Duration)
		**out = **in
	}
	if in.HandleInterval != nil {
		in, out := &in.HandleInterval, &out.HandleInterval
		*out = new(times.Duration)
		**out = **in
	}
	if in.RecoverInterval != nil {
		in, out := &in.RecoverInterval, &out.RecoverInterval
		*out = new(times.Duration)
		**out = **in
	}
	if in.Rules != nil {
		in, out := &in.Rules, &out.Rules
		*out = make([]*DetectActionRules, len(*in))
		for i := range *in {
			if (*in)[i] != nil {
				in, out := &(*in)[i], &(*out)[i]
				*out = new(DetectActionRules)
				(*in).DeepCopyInto(*out)
			}
		}
	}
	if in.RecoverRules != nil {
		in, out := &in.RecoverRules, &out.RecoverRules
		*out = make([]*DetectActionRules, len(*in))
		for i := range *in {
			if (*in)[i] != nil {
				in, out := &(*in)[i], &(*out)[i]
				*out = new(DetectActionRules)
				(*in).DeepCopyInto(*out)
			}
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new RuleCheckSpec.
func (in *RuleCheckSpec) DeepCopy() *RuleCheckSpec {
	if in == nil {
		return nil
	}
	out := new(RuleCheckSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *RuleCheckStatus) DeepCopyInto(out *RuleCheckStatus) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new RuleCheckStatus.
func (in *RuleCheckStatus) DeepCopy() *RuleCheckStatus {
	if in == nil {
		return nil
	}
	out := new(RuleCheckStatus)
	in.DeepCopyInto(out)
	return out
}
