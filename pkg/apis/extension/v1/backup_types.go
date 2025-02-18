package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	shpmetav1 "github.com/universityofadelaide/shepherd-operator/pkg/apis/meta/v1"
)

// BackupSpec defines the desired state of Backup
type BackupSpec struct {
	// Volumes which will be backed up.
	Volumes map[string]shpmetav1.SpecVolume `json:"volumes,omitempty"`
	// MySQL databases which will be backed up.
	MySQL map[string]shpmetav1.SpecMySQL `json:"mysql,omitempty"`
}

// BackupStatus defines the observed state of Backup
type BackupStatus struct {
	StartTime      *metav1.Time    `json:"startTime,omitempty"`
	CompletionTime *metav1.Time    `json:"completionTime,omitempty"`
	ResticID       string          `json:"resticId,omitempty"`
	Phase          shpmetav1.Phase `json:"phase"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Backup is the Schema for the backups API
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=.status.phase
type Backup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BackupSpec   `json:"spec,omitempty"`
	Status BackupStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// BackupList contains a list of Backup
type BackupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Backup `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Backup{}, &BackupList{})
}
