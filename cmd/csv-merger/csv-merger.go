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
	"strings"

	"github.com/ghodss/yaml"
	"github.com/imdario/mergo"
	"github.com/openstack-k8s-operators/openstack-operator/pkg/util"

	csvv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const csvMode = "CSV"

var validOutputModes = []string{csvMode}

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

var (
	outputMode     = flag.String("output-mode", csvMode, "Working mode: "+strings.Join(validOutputModes, "|"))
	openstackCsv   = flag.String("openstack-csv", "", "OpenStack CSV filename")
	keystoneCsv    = flag.String("keystone-csv", "", "Keystone CSV filename")
	mariadbCsv     = flag.String("mariadb-csv", "", "Mariadb CSV filename")
	rabbitmqCsv    = flag.String("rabbitmq-csv", "", "RabbitMQ CSV filename")
	infraCsv       = flag.String("infra-csv", "", "Infra CSV filename")
	ansibleEECsv   = flag.String("ansibleee-csv", "", "Ansible EE CSV filename")
	dataplaneCsv   = flag.String("dataplane-csv", "", "Data plane CSV filename")
	novaCsv        = flag.String("nova-csv", "", "Nova CSV filename")
	neutronCsv     = flag.String("neutron-csv", "", "Neutron CSV filename")
	glanceCsv      = flag.String("glance-csv", "", "Glance CSV filename")
	placementCsv   = flag.String("placement-csv", "", "Placement CSV filename")
	ovnCsv         = flag.String("ovn-csv", "", "OVN CSV filename")
	ovsCsv         = flag.String("ovs-csv", "", "OVS CSV filename")
	cinderCsv      = flag.String("cinder-csv", "", "Cinder CSV filename")
	csvOverrides   = flag.String("csv-overrides", "", "CSV like string with punctual changes that will be recursively applied (if possible)")
	visibleCRDList = flag.String("visible-crds-list", "openstackcontrolplanes.core.openstack.org,openstackclients.core.openstack.org",
		"Comma separated list of all the CRDs that should be visible in OLM console")
)

func getCSVBase(filename string) *csvv1alpha1.ClusterServiceVersion {
	csvBytes, err := os.ReadFile(filename)
	if err != nil {
		panic(err)
	}
	csvStruct := &csvv1alpha1.ClusterServiceVersion{}

	err = yaml.Unmarshal(csvBytes, csvStruct)
	if err != nil {
		panic(err)
	}
	return csvStruct
}

func main() {
	flag.Parse()

	switch *outputMode {
	case csvMode:

		csvs := []string{
			*keystoneCsv,
			*mariadbCsv,
			*rabbitmqCsv,
			*infraCsv,
			*ansibleEECsv,
			*dataplaneCsv,
			*novaCsv,
			*neutronCsv,
			*glanceCsv,
			*placementCsv,
			*ovnCsv,
			*ovsCsv,
			*cinderCsv,
		}

		// BaseCSV is built on the bundle/manifests/openstack-operator.clusterserviceversion.yaml from this repo
		csvBase := getCSVBase(*openstackCsv)
		csvExtended := clusterServiceVersionExtended{
			TypeMeta:   csvBase.TypeMeta,
			ObjectMeta: csvBase.ObjectMeta,
			Spec:       clusterServiceVersionSpecExtended{ClusterServiceVersionSpec: csvBase.Spec},
			Status:     csvBase.Status}

		installStrategyBase := csvBase.Spec.InstallStrategy.StrategySpec

		for _, csvFile := range csvs {
			if csvFile != "" {
				csvBytes, err := os.ReadFile(csvFile)
				if err != nil {
					panic(err)
				}

				csvStruct := &csvv1alpha1.ClusterServiceVersion{}

				err = yaml.Unmarshal(csvBytes, csvStruct)
				if err != nil {
					panic(err)
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

				csvBaseAlmString := csvExtended.Annotations["alm-examples"]
				csvStructAlmString := csvStruct.Annotations["alm-examples"]
				var baseAlmCrs []interface{}
				var structAlmCrs []interface{}
				if err = json.Unmarshal([]byte(csvBaseAlmString), &baseAlmCrs); err != nil {
					panic(err)
				}
				if err = json.Unmarshal([]byte(csvStructAlmString), &structAlmCrs); err == nil {
					//panic(err)
					baseAlmCrs = append(baseAlmCrs, structAlmCrs...)
				}
				almB, err := json.Marshal(baseAlmCrs)
				if err != nil {
					panic(err)
				}
				csvExtended.Annotations["alm-examples"] = string(almB)

			}
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
			panic(err)
		}
		csvExtended.Annotations["operators.operatorframework.io/internal-objects"] = string(hiddenCrdsJ)

		csvExtended.Spec.InstallStrategy.StrategyName = "deployment"
		csvExtended.Spec.InstallStrategy = csvv1alpha1.NamedInstallStrategy{
			StrategyName: "deployment",
			StrategySpec: installStrategyBase,
		}

		if *csvOverrides != "" {
			csvOBytes := []byte(*csvOverrides)

			csvO := &clusterServiceVersionExtended{}

			err := yaml.Unmarshal(csvOBytes, csvO)
			if err != nil {
				panic(err)
			}

			err = mergo.Merge(&csvExtended, csvO, mergo.WithOverride)
			if err != nil {
				panic(err)
			}

		}

		err = util.MarshallObject(csvExtended, os.Stdout)
		if err != nil {
			panic(err)
		}

	default:
		panic("Unsupported output mode: " + *outputMode)
	}

}
