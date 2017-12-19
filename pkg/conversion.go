/*
Copyright 2017 The Kedge Authors All rights reserved.

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

package pkg

import (
	"encoding/json"
	"fmt"
	"io/ioutil"

	log "github.com/Sirupsen/logrus"
	"github.com/go-openapi/spec"
	"k8s.io/apimachinery/pkg/openapi"
)

func ParseOpenAPIDefinition(filename string) (*openapi.OpenAPIDefinition, error) {

	content, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("cannot read file %q: %v\n", filename, err)
	}

	api := &openapi.OpenAPIDefinition{}
	err = json.Unmarshal(content, &api.Schema)
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling in OpenAPI definition: %v", err)
	}
	return api, nil
}

func MergeDefinitions(target, src *openapi.OpenAPIDefinition) {
	for k, v := range src.Schema.SchemaProps.Definitions {
		target.Schema.SchemaProps.Definitions[k] = v
	}
}

func Conversion(KedgeSpecLocation, KubernetesSchema, OpenShiftSchema string) error {
	defs, mapping, err := GenerateOpenAPIDefinitions(KedgeSpecLocation)
	if err != nil {
		return err
	}

	k8sApi, err := ParseOpenAPIDefinition(KubernetesSchema)
	if err != nil {
		return fmt.Errorf("kubernetes: %v", err)
	}

	osApi, err := ParseOpenAPIDefinition(OpenShiftSchema)
	if err != nil {
		return fmt.Errorf("openshift: %v", err)
	}

	MergeDefinitions(k8sApi, osApi)
	api := k8sApi

	defs = InjectKedgeSpec(api.Schema.SchemaProps.Definitions, defs, mapping)

	// add defs to openapi
	for k, v := range defs {
		api.Schema.SchemaProps.Definitions[k] = v
	}
	PrintJSONStdOut(api.Schema)
	return nil
}

func augmentProperties(s, t spec.Schema) spec.Schema {
	for k, v := range s.Properties {
		if _, ok := t.Properties[k]; !ok {
			t.Properties[k] = v
		}
	}
	t.Required = AddListUniqueItems(t.Required, s.Required)
	return t
}

func InjectKedgeSpec(apiDef spec.Definitions, defs spec.Definitions, mapping []Injection) spec.Definitions {
	for _, m := range mapping {
		defs[m.Target] = augmentProperties(apiDef[m.Source], defs[m.Target])

		// special case, where if the key is io.kedge.DeploymentSpec
		// ignore the required field called template
		switch m.Target {
		case "io.kedge.DeploymentSpecMod",
			"io.kedge.DeploymentConfigSpecMod",
			"io.kedge.JobSpecMod":
			v := defs[m.Target]
			var final []string
			for _, r := range v.Required {
				if r != "template" {
					final = append(final, r)
				}
			}
			v.Required = final
			defs[m.Target] = v
		}
	}
	return defs
}

func PrintJSONStdOut(v interface{}) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		log.Fatalln(err)
	}
	fmt.Println(string(b))
}
