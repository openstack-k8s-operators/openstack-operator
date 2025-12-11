package util //nolint:revive // util is an acceptable package name in this context

import (
	"encoding/json"
	"fmt"

	"github.com/openstack-k8s-operators/lib-common/modules/storage"
	yaml "gopkg.in/yaml.v3"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	helper "github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	util "github.com/openstack-k8s-operators/lib-common/modules/common/util"
)

// EEJob defines properties that will be applied to the Kubernetes jobs for Ansible EE pods
type EEJob struct {
	// PlaybookContents is an inline playbook contents that ansible will run on execution.
	PlaybookContents string `json:"playbookContents,omitempty"`
	// Playbook is the playbook that ansible will run on this execution, accepts path or FQN from collection
	Playbook string `json:"playbook,omitempty"`
	// Role is the role that ansible will run on this execution, accepts path or FQN from collection
	Role string `json:"role,omitempty"`
	// Image is the container image that will execute the ansible command
	Image string `json:"image,omitempty"`
	// Name is the name of the execution job
	Name string `json:"name,omitempty"`
	// Namespace - The kubernetes Namespace to create the job in
	Namespace string `json:"namespace,omitempty"`
	// EnvConfigMapName is the name of the k8s config map that contains the ansible env variables
	EnvConfigMapName string `json:"envConfigMapName,omitempty"`
	// CmdLine is the command line passed to ansible-runner
	CmdLine string `json:"cmdLine,omitempty"`
	// ServiceAccountName allows to specify what ServiceAccountName do we want the ansible execution run with. Without specifying,
	// it will run with default serviceaccount
	ServiceAccountName string `json:"serviceAccountName,omitempty"`
	// Args are the command plus the playbook executed by the image. If args is passed, Playbook is ignored.
	Args []string `json:"args,omitempty"`
	// NetworkAttachments is a list of NetworkAttachment resource names to expose the services to the given network
	NetworkAttachments []string `json:"networkAttachments,omitempty"`
	// PreserveJobs - do not delete jobs after they finished e.g. to check logs
	// PreserveJobs default: true
	PreserveJobs bool `json:"preserveJobs,omitempty"`
	// BackoffLimit allows to define the maximum number of retried executions (defaults to 6).
	BackoffLimit *int32 `json:"backoffLimit,omitempty"`
	// UID is the userid that will be used to run the container.
	UID int64 `json:"uid,omitempty"`
	// ExtraMounts containing conf files, credentials and inventories
	ExtraMounts []storage.VolMounts `json:"extraMounts,omitempty"`
	// InitContainers allows the passing of an array of containers that will be executed before the ansibleee execution itself
	InitContainers []corev1.Container `json:"initContainers,omitempty"`
	// DNSConfig allows to specify custom dnsservers and search domains
	DNSConfig *corev1.PodDNSConfig `json:"dnsConfig,omitempty"`
	// Extra vars to be passed to ansible process during execution. This can be used to override default values in plays.
	ExtraVars map[string]json.RawMessage `json:"extraVars,omitempty"`
	// Labels - Kubernetes labels to apply to the job
	Labels map[string]string `json:"labels,omitempty"`
	// Annotations - Kubernetes annotations to apply to the job
	Annotations map[string]string `json:"annotations,omitempty"`
	// Env is a list containing the environment variables to pass to the pod
	Env []corev1.EnvVar `json:"env,omitempty"`
	// NodeSelector to target subset of worker nodes running the ansible jobs
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
}

// JobForOpenStackAnsibleEE returns a Job object
func (a *EEJob) JobForOpenStackAnsibleEE(h *helper.Helper) (*batchv1.Job, error) {
	const (
		CustomPlaybook  string = "playbook.yaml"
		CustomInventory string = "/runner/inventory/inventory.yaml"
	)

	ls := labelsForOpenStackAnsibleEE(a.Labels)

	args := a.Args

	if len(args) == 0 {
		artifact := a.Playbook
		param := "-p"
		if len(artifact) == 0 {
			if len(a.PlaybookContents) > 0 {
				artifact = CustomPlaybook
			} else if len(a.Role) > 0 {
				artifact = a.Role
				param = "-r"
			} else {
				return nil, fmt.Errorf("no playbook, playbookContents or role specified")
			}
		}
		args = []string{"ansible-runner", "run", "/runner", param, artifact}
	}

	// ansible runner identifier
	// if the flag is set we use resource name as an argument
	// https://ansible-runner.readthedocs.io/en/stable/intro/#artifactdir
	if !util.StringInSlice("-i", args) && !util.StringInSlice("--ident", args) {
		identifier := a.Name
		args = append(args, []string{"-i", identifier}...)
	}

	podSpec := corev1.PodSpec{
		RestartPolicy: corev1.RestartPolicyNever,
		Containers: []corev1.Container{{
			ImagePullPolicy: corev1.PullAlways,
			Image:           a.Image,
			Name:            a.Name,
			Args:            args,
			Env:             a.Env,
		}},
	}

	if len(a.NodeSelector) > 0 {
		podSpec.NodeSelector = a.NodeSelector
	}

	if a.DNSConfig != nil {
		podSpec.DNSConfig = a.DNSConfig
		podSpec.DNSPolicy = "None"
	}

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:        a.Name,
			Namespace:   a.Namespace,
			Annotations: a.Annotations,
			Labels:      ls,
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: a.BackoffLimit,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: a.Annotations,
					Labels:      ls,
				},
				Spec: podSpec,
			},
		},
	}

	// Populate hash
	hashes := make(map[string]string)

	if len(a.InitContainers) > 0 {
		job.Spec.Template.Spec.InitContainers = a.InitContainers
	}
	if len(a.ServiceAccountName) > 0 {
		job.Spec.Template.Spec.ServiceAccountName = a.ServiceAccountName
	}

	if len(a.Role) > 0 {
		setRunnerEnvVar(h, "RUNNER_ROLE", a.Role, "role", job, hashes)
	}
	if len(a.PlaybookContents) > 0 {
		setRunnerEnvVar(h, "RUNNER_PLAYBOOK", a.PlaybookContents, "playbookContents", job, hashes)
	} else if len(a.Playbook) > 0 {
		// As we set "playbook.yaml" as default
		// we need to ensure that PlaybookContents is empty before adding playbook
		setRunnerEnvVar(h, "RUNNER_PLAYBOOK", a.Playbook, "playbooks", job, hashes)
	}

	if len(a.CmdLine) > 0 {
		setRunnerEnvVar(h, "RUNNER_CMDLINE", a.CmdLine, "cmdline", job, hashes)
	}
	if len(a.Labels["deployIdentifier"]) > 0 {
		hashes["deployIdentifier"] = a.Labels["deployIdentifier"]
	}

	errMounts := a.addMounts(job)
	if errMounts != nil {
		return nil, errMounts
	}
	a.addEnvFrom(job)

	// if we have any extra vars for ansible to use set them in the RUNNER_EXTRA_VARS
	if len(a.ExtraVars) > 0 {
		extraVarsMap := make(map[string]interface{})
		for variable, rawValue := range a.ExtraVars {
			var tmp interface{}
			err := yaml.Unmarshal(rawValue, &tmp)
			if err != nil {
				return nil, err
			}
			extraVarsMap[variable] = tmp
		}

		yamlBytes, err := yaml.Marshal(extraVarsMap)
		if err != nil {
			return nil, err
		}

		setRunnerEnvVar(h, "RUNNER_EXTRA_VARS", string(yamlBytes), "extraVars", job, hashes)
	}

	hashPodSpec(h, podSpec, hashes)

	return job, nil
}

// labelsForOpenStackAnsibleEE returns the labels for ansible execution job.
func labelsForOpenStackAnsibleEE(labels map[string]string) map[string]string {
	ls := map[string]string{
		"app": "openstackansibleee",
	}
	for key, val := range labels {
		ls[key] = val
	}
	return ls
}

func (a *EEJob) addEnvFrom(job *batchv1.Job) {
	// Add optional config map
	optional := true
	job.Spec.Template.Spec.Containers[0].EnvFrom = []corev1.EnvFromSource{
		{
			ConfigMapRef: &corev1.ConfigMapEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: a.EnvConfigMapName},
				Optional:             &optional,
			},
		},
	}
}

func (a *EEJob) addMounts(job *batchv1.Job) error {
	var volumeMounts []corev1.VolumeMount
	var volumes []storage.Volume
	// ExtraMounts propagation: for each volume defined in the top-level CR
	// the propagation function provided by lib-common/modules/storage is
	// called, and the resulting corev1.Volumes and corev1.Mounts are added
	// to the main list defined by the ansible operator
	for _, exv := range a.ExtraMounts {
		for _, vol := range exv.Propagate([]storage.PropagationType{storage.Compute}) {
			volumes = append(volumes, vol.Volumes...)
			volumeMounts = append(volumeMounts, vol.Mounts...)
		}
	}

	job.Spec.Template.Spec.Containers[0].VolumeMounts = volumeMounts

	var coreVols []corev1.Volume
	for _, vol := range volumes {
		coreVol, errVol := vol.ToCoreVolume()
		if errVol != nil {
			return errVol
		}
		coreVols = append(coreVols, *coreVol)
	}
	job.Spec.Template.Spec.Volumes = coreVols
	return nil
}

func hashPodSpec(
	h *helper.Helper,
	podSpec corev1.PodSpec,
	hashes map[string]string,
) {
	var err error
	spec, _ := podSpec.Marshal()
	hashes["podspec"], err = calculateHash(string(spec))
	if err != nil {
		h.GetLogger().Error(err, "Error calculating the PodSpec hash")
	}
}

// set value of runner environment variable and compute the hash
func setRunnerEnvVar(
	helper *helper.Helper,
	varName string,
	varValue string,
	hashType string,
	job *batchv1.Job,
	hashes map[string]string,
) {
	var envVar corev1.EnvVar
	var err error
	envVar.Name = varName
	envVar.Value = "\n" + varValue + "\n\n"
	job.Spec.Template.Spec.Containers[0].Env = append(job.Spec.Template.Spec.Containers[0].Env, envVar)
	hashes[hashType], err = calculateHash(varValue)
	if err != nil {
		helper.GetLogger().Error(err, "Error calculating the hash")
	}
}

func calculateHash(envVar string) (string, error) {
	hash, err := util.ObjectHash(envVar)
	if err != nil {
		return "", err
	}
	return hash, nil
}
