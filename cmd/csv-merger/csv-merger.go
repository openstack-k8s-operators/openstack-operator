/*
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 * Copyright 2022 Red Hat, Inc.
 *
 */

package main

import (
	"encoding/json"
	"flag"
	"os"
	"runtime/debug"
	"strings"

	"github.com/ghodss/yaml"
	"github.com/imdario/mergo"
	"github.com/openstack-k8s-operators/openstack-operator/pkg/util"

	semver "github.com/blang/semver/v4"
	"github.com/operator-framework/api/pkg/lib/version"
	csvv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TODO: get rid of this if/when RelatedImages officially appears in github.com/operator-framework/api/pkg/operators/v1alpha1/
type relatedImage struct {
	Name string `json:"name"`
	Ref  string `json:"image"`
}

type clusterServiceVersionSpecExtended struct {
	csvv1alpha1.ClusterServiceVersionSpec
	RelatedImages []relatedImage `json:"relatedImages,omitempty"`
}

type clusterServiceVersionExtended struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Spec   clusterServiceVersionSpecExtended       `json:"spec"`
	Status csvv1alpha1.ClusterServiceVersionStatus `json:"status"`
}

func getEnvsFromFile(filename string) []v1.EnvVar {
	csvBytes, err := os.ReadFile(filename)
	if err != nil {
		exitWithError(err.Error())
	}
	envVarList := &[]v1.EnvVar{}

	err = yaml.Unmarshal(csvBytes, envVarList)
	if err != nil {
		exitWithError(err.Error())
	}
	return *envVarList
}

func writeEnvToYaml(filename string, data interface{}) {
	yamlBytes, err := yaml.Marshal(data)
	if err != nil {
		exitWithError(err.Error())
	}

	file, err := os.Create(filename)
	if err != nil {
		exitWithError(err.Error())
	}
	defer file.Close()
	_, err = file.Write(yamlBytes)
	if err != nil {
		exitWithError(err.Error())
	}
}

func exitWithError(msg string) {
	os.Stderr.WriteString(msg)
	debug.PrintStack()
	os.Exit(1)
}

var (
	baseCsv        = flag.String("base-csv", "", "Base CSV filename")
	keystoneCsv    = flag.String("keystone-csv", "", "Keystone CSV filename")
	mariadbCsv     = flag.String("mariadb-csv", "", "Mariadb CSV filename")
	rabbitmqCsv    = flag.String("rabbitmq-csv", "", "RabbitMQ CSV filename")
	infraCsv       = flag.String("infra-csv", "", "Infra CSV filename")
	ansibleEECsv   = flag.String("ansibleee-csv", "", "Ansible EE CSV filename")
	dataplaneCsv   = flag.String("dataplane-csv", "", "Data plane CSV filename")
	novaCsv        = flag.String("nova-csv", "", "Nova CSV filename")
	neutronCsv     = flag.String("neutron-csv", "", "Neutron CSV filename")
	glanceCsv      = flag.String("glance-csv", "", "Glance CSV filename")
	ironicCsv      = flag.String("ironic-csv", "", "Ironic CSV filename")
	baremetalCsv   = flag.String("baremetal-csv", "", "Baremetal CSV filename")
	manilaCsv      = flag.String("manila-csv", "", "Manila CSV filename")
	placementCsv   = flag.String("placement-csv", "", "Placement CSV filename")
	telemetryCsv   = flag.String("telemetry-csv", "", "Telemetry CSV filename")
	ovnCsv         = flag.String("ovn-csv", "", "OVN CSV filename")
	ovsCsv         = flag.String("ovs-csv", "", "OVS CSV filename")
	cinderCsv      = flag.String("cinder-csv", "", "Cinder CSV filename")
	horizonCsv     = flag.String("horizon-csv", "", "Horizon CSV filename")
	csvOverrides   = flag.String("csv-overrides", "", "CSV like string with punctual changes that will be recursively applied (if possible)")
	importEnvFiles = flag.String("import-env-files", "", "Comma separated list of file names to read default operator ENVs from. Used for inter-bundle ENV merging.")
	exportEnvFile  = flag.String("export-env-file", "", "Name the external file to write operator ENVs to. Used for inter-bundle ENV merging.")
	almExamples    = flag.Bool("alm-examples", false, "Merge almExamples into the CSV")
	visibleCRDList = flag.String("visible-crds-list", "openstackcontrolplanes.core.openstack.org,openstackdataplanes.dataplane.openstack.org,openstackdataplaneroles.dataplane.openstack.org,openstackdataplanenodes.dataplane.openstack.org",
		"Comma separated list of all the CRDs that should be visible in OLM console")
)

func getCSVBase(filename string) *csvv1alpha1.ClusterServiceVersion {
	csvBytes, err := os.ReadFile(filename)
	if err != nil {
		exitWithError(err.Error())
	}
	csvStruct := &csvv1alpha1.ClusterServiceVersion{}

	err = yaml.Unmarshal(csvBytes, csvStruct)
	if err != nil {
		exitWithError(err.Error())
	}
	return csvStruct
}

func main() {
	flag.Parse()

	csvs := []string{
		*keystoneCsv,
		*mariadbCsv,
		*rabbitmqCsv,
		*infraCsv,
		*ansibleEECsv,
		*dataplaneCsv,
		*novaCsv,
		*neutronCsv,
		*manilaCsv,
		*glanceCsv,
		*ironicCsv,
		*baremetalCsv,
		*placementCsv,
		*telemetryCsv,
		*ovnCsv,
		*ovsCsv,
		*cinderCsv,
		*horizonCsv,
	}

	csvVersion := os.Getenv("CSV_VERSION")

	// BaseCSV is built on the bundle/manifests/openstack-operator.clusterserviceversion.yaml from this repo
	csvBase := getCSVBase(*baseCsv)
	csvExtended := clusterServiceVersionExtended{
		TypeMeta:   csvBase.TypeMeta,
		ObjectMeta: csvBase.ObjectMeta,
		Spec:       clusterServiceVersionSpecExtended{ClusterServiceVersionSpec: csvBase.Spec},
		Status:     csvBase.Status}

	installStrategyBase := csvBase.Spec.InstallStrategy.StrategySpec

	envVarList := []v1.EnvVar{}
	if len(*importEnvFiles) > 0 {
		for _, filename := range strings.Split(*importEnvFiles, ",") {
			envVarList = append(envVarList, getEnvsFromFile(filename)...)
		}
	}
	for _, csvFile := range csvs {
		if csvFile != "" {
			csvBytes, err := os.ReadFile(csvFile)
			if err != nil {
				exitWithError(err.Error())
			}

			csvStruct := &csvv1alpha1.ClusterServiceVersion{}

			err = yaml.Unmarshal(csvBytes, csvStruct)
			if err != nil {
				exitWithError(err.Error())
			}

			// 1. We need to add the "env" section from this Service Operator deployment in case there
			// are default values configured there that are needed for use with defaulting webhooks
			//
			// - DeploymentSpecs[0] is always the base deployment for OpenStack Operator
			// - Container at index 1 in DeploymentSpecs[0].Spec.Template.Spec.Containers list is
			//   always the OpenStack Operator controller-manager
			// - We need to find the Service Operator's controller-manager container in
			//   csvStruct.Spec.InstallStrategy.StrategySpec.DeploymentSpecs[0].Spec.Template.Spec.Containers
			//
			// TODO: What about "env" list keys that overlap between Service Operators (i.e. non-unique
			//       names)?
			//
			// 2. We also need to inject "ENABLE_WEBHOOKS=false" into the env vars for the Service Operators'
			//    deployments, and then remove their webhook server's cert's volume mount

			for index, container := range csvStruct.Spec.InstallStrategy.StrategySpec.DeploymentSpecs[0].Spec.Template.Spec.Containers {
				// Copy env vars from the Service Operator into the OpenStack Operator
				if container.Name == "manager" {
					envVarList = append(envVarList, container.Env...)

					// Now we also need to turn off any "internal" webhooks that belong to the service
					// operator, as we are now using "external" webhooks that live in the OpenStack
					// operator.  These "external" webhooks will eventually call the mutating/validating
					// logic that was previously housed within the "internal" Service Operator webhook
					// logic.
					container.Env = append(container.Env,
						v1.EnvVar{
							Name:  "ENABLE_WEBHOOKS",
							Value: "false",
						},
					)

					// And finally we need to remove the webhook server's cert volume mount
					for volMountIndex, volMount := range container.VolumeMounts {
						if volMount.Name == "cert" {
							container.VolumeMounts[volMountIndex] = container.VolumeMounts[len(container.VolumeMounts)-1]
							container.VolumeMounts = container.VolumeMounts[:len(container.VolumeMounts)-1]
							// Found the target mount, so stop iterating
							break
						}
					}

					// Need to replace the container in the Deployment since this local variable is a copy
					csvStruct.Spec.InstallStrategy.StrategySpec.DeploymentSpecs[0].Spec.Template.Spec.Containers[index] = container

					// We found the controller-manager container, so no need to continue iterating
					break
				}
			}

			installStrategyBase.DeploymentSpecs = append(installStrategyBase.DeploymentSpecs, csvStruct.Spec.InstallStrategy.StrategySpec.DeploymentSpecs...)
			installStrategyBase.ClusterPermissions = append(installStrategyBase.ClusterPermissions, csvStruct.Spec.InstallStrategy.StrategySpec.ClusterPermissions...)
			installStrategyBase.Permissions = append(installStrategyBase.Permissions, csvStruct.Spec.InstallStrategy.StrategySpec.Permissions...)

			for _, owned := range csvStruct.Spec.CustomResourceDefinitions.Owned {
				csvExtended.Spec.CustomResourceDefinitions.Owned = append(
					csvExtended.Spec.CustomResourceDefinitions.Owned,
					csvv1alpha1.CRDDescription{
						Name:        owned.Name,
						Version:     owned.Version,
						Kind:        owned.Kind,
						Description: owned.Description,
						DisplayName: owned.DisplayName,
					},
				)
			}

			if *almExamples {
				csvBaseAlmString := csvExtended.Annotations["alm-examples"]
				csvStructAlmString := csvStruct.Annotations["alm-examples"]
				var baseAlmCrs []interface{}
				var structAlmCrs []interface{}
				if err = json.Unmarshal([]byte(csvBaseAlmString), &baseAlmCrs); err != nil {
					print(csvBaseAlmString)
					exitWithError(err.Error())
				}
				if err = json.Unmarshal([]byte(csvStructAlmString), &structAlmCrs); err == nil {
					//panic(err)
					baseAlmCrs = append(baseAlmCrs, structAlmCrs...)
				}
				almB, err := json.Marshal(baseAlmCrs)
				if err != nil {
					exitWithError(err.Error())
				}
				csvExtended.Annotations["alm-examples"] = string(almB)
			}

		}
	}
	if len(*exportEnvFile) > 0 {
		writeEnvToYaml(*exportEnvFile, envVarList)
	} else {
		installStrategyBase.DeploymentSpecs[0].Spec.Template.Spec.Containers[1].Env = append(
			// OpenStack Operator controller-manager container env vars
			installStrategyBase.DeploymentSpecs[0].Spec.Template.Spec.Containers[1].Env,
			// Service Operator controller-manager container env vars
			envVarList...,
		)
	}

	// by default we hide all CRDs in the Console
	hiddenCrds := []string{}
	visibleCrds := strings.Split(*visibleCRDList, ",")
	for _, owned := range csvExtended.Spec.CustomResourceDefinitions.Owned {
		found := false
		for _, name := range visibleCrds {
			if owned.Name == name {
				found = true
			}
		}
		if !found {
			hiddenCrds = append(
				hiddenCrds,
				owned.Name,
			)
		}
	}
	hiddenCrdsJ, err := json.Marshal(hiddenCrds)
	if err != nil {
		exitWithError(err.Error())
	}
	csvExtended.Annotations["operators.operatorframework.io/internal-objects"] = string(hiddenCrdsJ)

	csvExtended.Spec.InstallStrategy.StrategyName = "deployment"
	csvExtended.Spec.InstallStrategy = csvv1alpha1.NamedInstallStrategy{
		StrategyName: "deployment",
		StrategySpec: installStrategyBase,
	}

	if csvVersion != "" {
		csvExtended.Spec.Version = version.OperatorVersion{
			Version: semver.MustParse(csvVersion),
		}
		csvExtended.Name = strings.Replace(csvExtended.Name, "0.0.1", csvVersion, 1)
	}

	if *csvOverrides != "" {
		csvOBytes := []byte(*csvOverrides)

		csvO := &clusterServiceVersionExtended{}

		err := yaml.Unmarshal(csvOBytes, csvO)
		if err != nil {
			exitWithError(err.Error())
		}

		err = mergo.Merge(&csvExtended, csvO, mergo.WithOverride)
		if err != nil {
			exitWithError(err.Error())
		}

	}

	err = util.MarshallObject(csvExtended, os.Stdout)
	if err != nil {
		exitWithError(err.Error())
	}

}
