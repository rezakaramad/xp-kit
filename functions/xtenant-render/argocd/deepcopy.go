package argocd

import "k8s.io/apimachinery/pkg/runtime"

// DeepCopyInto copies all fields of in into out.
func (in *Application) DeepCopyInto(out *Application) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
}

// DeepCopy returns a deep copy of the Application.
func (in *Application) DeepCopy() *Application {
	if in == nil {
		return nil
	}
	out := new(Application)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject returns a deep copy of the Application as a runtime.Object.
func (in *Application) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto copies all fields of in into out.
func (in *ApplicationList) DeepCopyInto(out *ApplicationList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		out.Items = make([]Application, len(in.Items))
		for i := range in.Items {
			in.Items[i].DeepCopyInto(&out.Items[i])
		}
	}
}

// DeepCopy returns a deep copy of the ApplicationList.
func (in *ApplicationList) DeepCopy() *ApplicationList {
	if in == nil {
		return nil
	}
	out := new(ApplicationList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject returns a deep copy of the ApplicationList as a runtime.Object.
func (in *ApplicationList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto copies all fields of in into out.
func (in *ApplicationSpec) DeepCopyInto(out *ApplicationSpec) {
	*out = *in
	out.Source = in.Source
	if in.Source.Helm != nil {
		helm := *in.Source.Helm
		out.Source.Helm = &helm
	}
	out.Destination = in.Destination
	if in.SyncPolicy != nil {
		sp := new(SyncPolicy)
		if in.SyncPolicy.Automated != nil {
			auto := *in.SyncPolicy.Automated
			sp.Automated = &auto
		}
		out.SyncPolicy = sp
	}
}
