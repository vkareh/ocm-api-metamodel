/*
Copyright (c) 2019 Red Hat, Inc.

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

package javascript

import (
	"fmt"

	"github.com/openshift-online/ocm-api-metamodel/pkg/concepts"
	"github.com/openshift-online/ocm-api-metamodel/pkg/names"
	"github.com/openshift-online/ocm-api-metamodel/pkg/nomenclator"
	"github.com/openshift-online/ocm-api-metamodel/pkg/reporter"
)

// TypesGeneratorBuilder is an object used to configure and build the types generator. Don't create
// instances directly, use the NewTypesGenerator function instead.
type TypesGeneratorBuilder struct {
	reporter *reporter.Reporter
	model    *concepts.Model
	output   string
	packages *PackagesCalculator
	names    *NamesCalculator
	types    *TypesCalculator
}

// TypesGenerator JavaScript types for the model types. Don't create instances directly, use the builder
// instead.
type TypesGenerator struct {
	reporter *reporter.Reporter
	errors   int
	model    *concepts.Model
	output   string
	packages *PackagesCalculator
	names    *NamesCalculator
	types    *TypesCalculator
	buffer   *Buffer
}

// NewTypesGenerator creates a new builder for types generators.
func NewTypesGenerator() *TypesGeneratorBuilder {
	return &TypesGeneratorBuilder{}
}

// Reporter sets the object that will be used to report information about the generation process,
// including errors.
func (b *TypesGeneratorBuilder) Reporter(value *reporter.Reporter) *TypesGeneratorBuilder {
	b.reporter = value
	return b
}

// Model sets the model that will be used by the types generator.
func (b *TypesGeneratorBuilder) Model(value *concepts.Model) *TypesGeneratorBuilder {
	b.model = value
	return b
}

// Output sets import path of the output package.
func (b *TypesGeneratorBuilder) Output(value string) *TypesGeneratorBuilder {
	b.output = value
	return b
}

// Packages sets the object that will be used to calculate package names.
func (b *TypesGeneratorBuilder) Packages(value *PackagesCalculator) *TypesGeneratorBuilder {
	b.packages = value
	return b
}

// Names sets the object that will be used to calculate names.
func (b *TypesGeneratorBuilder) Names(value *NamesCalculator) *TypesGeneratorBuilder {
	b.names = value
	return b
}

// Types sets the object that will be used to calculate types.
func (b *TypesGeneratorBuilder) Types(value *TypesCalculator) *TypesGeneratorBuilder {
	b.types = value
	return b
}

// Build checks the configuration stored in the builder and, if it is correct, creates a new
// types generator using it.
func (b *TypesGeneratorBuilder) Build() (generator *TypesGenerator, err error) {
	// Check that the mandatory parameters have been provided:
	if b.reporter == nil {
		err = fmt.Errorf("reporter is mandatory")
		return
	}
	if b.model == nil {
		err = fmt.Errorf("model is mandatory")
		return
	}
	if b.output == "" {
		err = fmt.Errorf("output is mandatory")
		return
	}
	if b.packages == nil {
		err = fmt.Errorf("packages calculator is mandatory")
		return
	}
	if b.names == nil {
		err = fmt.Errorf("names calculator is mandatory")
		return
	}
	if b.types == nil {
		err = fmt.Errorf("types calculator is mandatory")
		return
	}

	// Create the generator:
	generator = &TypesGenerator{
		reporter: b.reporter,
		model:    b.model,
		output:   b.output,
		packages: b.packages,
		names:    b.names,
		types:    b.types,
	}

	return
}

// Run executes the code generator.
func (g *TypesGenerator) Run() error {
	var err error

	// Generate the JavaScript types:
	for _, service := range g.model.Services() {
		for _, version := range service.Versions() {
			// Generate the version metadata type:
			err := g.generateVersionMetadataTypeFile(version)
			if err != nil {
				return err
			}

			// Generate the JavaScript types that correspond to model types:
			for _, typ := range version.Types() {
				switch {
				case typ.IsEnum() || typ.IsStruct():
					err = g.generateTypeFile(typ)
				}
				if err != nil {
					return err
				}
			}
		}
	}

	// Check if there were errors:
	if g.errors > 0 {
		if g.errors > 1 {
			err = fmt.Errorf("there were %d errors", g.errors)
		} else {
			err = fmt.Errorf("there was 1 error")
		}
		return err
	}

	return nil
}

func (g *TypesGenerator) generateVersionMetadataTypeFile(version *concepts.Version) error {
	var err error

	// Calculate the package and file name:
	pkgName := g.packages.VersionPackage(version)
	fileName := g.metadataFile()

	// Create the buffer for the generated code:
	g.buffer, err = NewBuffer().
		Reporter(g.reporter).
		Output(g.output).
		Packages(g.packages).
		Package(pkgName).
		File(fileName).
		Build()
	if err != nil {
		return err
	}

	// Generate the source:
	g.generateVersionMetadataTypeSource(version)

	// Write the generated code:
	return g.buffer.Write()
}

func (g *TypesGenerator) generateVersionMetadataTypeSource(version *concepts.Version) {
	g.buffer.Emit(`
// Metadata contains the version metadata.
export class Metadata {
	constructor(props) {
		this.serverVersion = props.serverVersion;
	}

	// ServerVersion returns the version of the server.
	get ServerVersion() {
		return this.serverVersion;
	}
}
		`,
	)
}

func (g *TypesGenerator) generateTypeFile(typ *concepts.Type) error {
	var err error

	// Calculate the package and file name:
	pkgName := g.packages.VersionPackage(typ.Owner())
	fileName := g.typeFile(typ)

	// Create the buffer for the generated code:
	g.buffer, err = NewBuffer().
		Reporter(g.reporter).
		Output(g.output).
		Packages(g.packages).
		Package(pkgName).
		File(fileName).
		Function("enumName", g.types.EnumName).
		Function("fieldName", g.fieldName).
		Function("fieldType", g.fieldType).
		Function("getterName", g.getterName).
		Function("getterType", g.getterType).
		Function("listName", g.listName).
		Function("objectName", g.objectName).
		Function("valueName", g.valueName).
		Function("valueTag", g.valueTag).
		Function("zeroValue", g.types.ZeroValue).
		Build()
	if err != nil {
		return err
	}

	// Generate the source:
	g.generateTypeSource(typ)

	// Write the generated code:
	return g.buffer.Write()
}

func (g *TypesGenerator) generateTypeSource(typ *concepts.Type) {
	switch {
	case typ.IsEnum():
		g.generateEnumTypeSource(typ)
	case typ.IsStruct():
		g.generateStructTypeSource(typ)
	}
}

func (g *TypesGenerator) generateEnumTypeSource(typ *concepts.Type) {
	g.buffer.Emit(`
{{ $enumName := enumName .Type }}

// {{ $enumName }} represents the values of the '{{ .Type.Name }}' enumerated type.
export const {{ $enumName }} {
{{ range .Type.Values }}
	{{ lineComment .Doc }}
	{{ valueName . }}: "{{ valueTag . }}",
{{ end }}
};
		`,
		"Type", typ,
	)
}

func (g *TypesGenerator) generateStructTypeSource(typ *concepts.Type) {
	g.buffer.Emit(`
{{ $objectName := objectName .Type }}
{{ $listName := listName .Type }}

{{ if .Type.IsClass }}
// {{ $objectName }}Kind is the name of the type used to represent objects
// of type '{{ .Type.Name }}'.
export const {{ $objectName }}Kind = "{{ $objectName }}";

// {{ $objectName }}LinkKind is the name of the type used to represent links
// to objects of type '{{ .Type.Name }}'.
export const {{ $objectName }}LinkKind = "{{ $objectName }}Link";
{{ end }}

// {{ $objectName }} represents the values of the '{{ .Type.Name }}' type.
//
{{ lineComment .Type.Doc }}
export class {{ $objectName }} {
	constructor(props) {
	{{ if .Type.IsClass }}
		this.id = props.id;
		this.href = props.href;
		this.link = props.link;
	{{ end }}
	{{ range .Type.Attributes }}
		this.{{ fieldName . }} = props.{{ fieldName . }};
	{{ end }}
	}

{{ if .Type.IsClass }}
	// Kind returns the name of the type of the object.
	get Kind() {
		if (this.link) {
			return {{ $objectName }}LinkKind;
		}
		return {{ $objectName }}Kind;
	}

	// ID returns the identifier of the object.
	get ID() {
		return this.id;
	}

	// HREF returns the link to the object.
	get HREF() {
		return this.href;
	}

	// Link returns true iif this is a link.
	isLink() {
		return !!this.link;
	}
{{ end }}

	// Empty returns true if the object is empty, i.e. no attribute has a value.
	isEmpty() {
		return (
		{{ if .Type.IsClass }}
			!this.id &&
		{{ end }}
		{{ range .Type.Attributes }}
			{{ $fieldName := fieldName . }}
		{{ if .Type.IsScalar }}
			!this.{{ $fieldName }} &&
		{{ else if .Type.IsList }}
			this.{{ $fieldName }}.length === 0 &&
		{{ else if .Type.IsMap }}
			Object.entries(this.{{ $fieldName }}).length === 0 &&
		{{ end }}
		{{ end }}
			true);
	}

{{ range .Type.Attributes }}
	{{ $attributeType := .Type.Name.String }}
	{{ $fieldName := fieldName . }}
	{{ $getterName := getterName . }}
	{{ $getterType := getterType . }}

	// {{ $getterName }} returns the value of the '{{ .Name }}' attribute.
	//
	{{ lineComment .Doc }}
	get {{ $getterName }}() {
		return this.{{ $fieldName }};
	}
{{ end }}
}

// {{ $listName }}Kind is the name of the type used to represent list of objects of
// type '{{ .Type.Name }}'.
export const {{ $listName }}Kind = "{{ $listName }}";

// {{ $listName }}LinkKind is the name of the type used to represent links to list
// of objects of type '{{ .Type.Name }}'.
export const {{ $listName }}LinkKind = "{{ $listName }}Link";

// {{ $listName }} is a list of values of the '{{ .Type.Name }}' type.
export class {{ $listName }} {
	constructor(props) {
		this.href = props.href;
		this.link = props.link;
		this.items = props.{{ $objectName }} || [];
	}

{{ if .Type.IsClass }}
	// Kind returns the name of the type of the object.
	get Kind() {
		if (this.link) {
			return {{ $listName }}LinkKind;
		}
		return {{ $listName }}Kind;
	}

	// HREF returns the link to the list.
	get HREF() {
		return this.href;
	}

	// Link returns true iif this is a link.
	isLink() {
		return !!this.link;
	}
{{ end }}

	// Len returns the length of the list.
	get length() {
		return this.items.length || 0;
	}

	// Empty returns true if the list is empty.
	isEmpty() {
		return !this.items.length;
	}

	// Get returns the item of the list with the given index. If there is no item with
	// that index it returns undefined.
	Get(i) {
		if (i >= this.items.length) {
			return undefined;
		}
		return this.items[i];
	}

	// Slice returns a slice containing the items of the list. The returned slice is a
	// shallow copy of the one used internally, so it can be modified without affecting
	// the internal representation.
	//
	// If you don't need to modify the returned slice consider using the Each or Range
	// functions, as they don't need to allocate a new slice.
	Slice() {
		return this.items.slice();
	}

	// Each runs the given function for each item of the list, in order. If the function
	// returns false the iteration stops, otherwise it continues until all the elements
	// of the list have been processed.
	Each(callback) {
		this.items.every(item => callback(item));
	}

	// Range runs the given function for each index and item of the list, in order. If
	// the function returns false the iteration stops, otherwise it continues till all
	// the elements of the list have been processed.
	Range(callback) {
		this.items.every((item, index) => callback(index, item));
	}
}
	`, "Type", typ)
}

func (g *TypesGenerator) metadataFile() string {
	return g.names.File(names.Cat(nomenclator.Metadata, nomenclator.Type))
}

func (g *TypesGenerator) typeFile(typ *concepts.Type) string {
	return g.names.File(names.Cat(typ.Name(), nomenclator.Type))
}

func (g *TypesGenerator) fieldName(attribute *concepts.Attribute) string {
	return g.names.Private(attribute.Name())
}

func (g *TypesGenerator) getterType(attribute *concepts.Attribute) *TypeReference {
	var ref *TypeReference
	typ := attribute.Type()
	switch {
	case typ.IsScalar():
		ref = g.types.ValueReference(typ)
	case typ.IsStruct():
		ref = g.types.NullableReference(typ)
	case typ.IsList():
		if attribute.Link() {
			ref = g.types.ListReference(typ)
		} else {
			ref = g.types.NullableReference(typ)
		}
	case typ.IsMap():
		ref = g.types.NullableReference(typ)
	}
	if ref == nil {
		g.reporter.Errorf(
			"Don't know how to calculate getter type for attribute '%s'",
			attribute,
		)
		ref = &TypeReference{}
	}
	return ref
}

func (g *TypesGenerator) objectName(typ *concepts.Type) string {
	return g.names.Public(typ.Name())
}

func (g *TypesGenerator) valueName(value *concepts.EnumValue) string {
	return g.names.Public(names.Cat(value.Type().Name(), value.Name()))
}

func (g *TypesGenerator) valueTag(value *concepts.EnumValue) string {
	return value.Name().String()
}

func (g *TypesGenerator) getterName(attribute *concepts.Attribute) string {
	return g.names.Public(attribute.Name())
}

func (g *TypesGenerator) fieldType(attribute *concepts.Attribute) *TypeReference {
	var ref *TypeReference
	typ := attribute.Type()
	switch {
	case typ.IsScalar():
		ref = g.types.NullableReference(typ)
	case typ.IsStruct():
		ref = g.types.NullableReference(typ)
	case typ.IsList():
		if attribute.Link() {
			ref = g.types.ListReference(typ)
		} else {
			ref = g.types.NullableReference(typ)
		}
	case typ.IsMap():
		ref = g.types.NullableReference(typ)
	}
	if ref == nil {
		g.reporter.Errorf(
			"Don't know how to calculate field type for attribute '%s'",
			attribute,
		)
		ref = &TypeReference{}
	}
	return ref
}

func (g *TypesGenerator) listName(typ *concepts.Type) string {
	name := names.Cat(typ.Name(), nomenclator.List)
	return g.names.Public(name)
}
