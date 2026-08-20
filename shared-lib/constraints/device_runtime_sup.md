# Specification Update Proposal: Device-specific runtime matching via device constraints

## Owner

@phil-abb

## Summary

This SUP proposes a generic device constraints mechanism that allows application suppliers to target devices using supplier-defined metadata without requiring the Margo specification to standardize every device characteristic or runtime-specific detail.

The proposal adds an optional `labels` dictionary to the Device Capabilities document and replaces the current `requiredResources` field in the deployment profile with a new `deviceConstraints` field in both the Application Description and Application Deployment documents. Together, these additions allow a Workload Fleet Manager or gateway to match a deployment to devices that satisfy supplier-agreed constraints such as vendor, deployment type, operating system, custom runtime support, and capacity constraints like minimum CPU, memory, and storage requirements.

This is an alternative proposal to the [original SUP for supporting device specific runtimes](https://github.com/margo/specification-enhancements/blob/device-specific-runtime/proposals/device-specific-runtime.md) and proposes a more general approach for solving the same problem.

## Reason for proposal

This SUP addresses [issue 93](https://github.com/margo/specification/issues/93), which asks how Margo can support deployments to devices with custom or device-specific runtimes without forcing the core specification to standardize every such runtime.

The current Margo specification defines standardized device capabilities and standardized deployment artifacts, but it does not provide a generic way for suppliers to express:

1. additional device characteristics that matter to cooperating suppliers but are outside Margo's interoperability scope
2. deployment-time constraints that tell a workload fleet manager or gateway which devices are acceptable targets

Without a generic matching mechanism, each new runtime or device-specific deployment requirement tends to push runtime-specific fields into the core specification. That approach does not scale. It also makes Margo responsible for describing proprietary or ecosystem-specific behavior that Margo cannot reasonably standardize or compliance-test.

This proposal improves the specification by separating two concerns:

1. Margo continues to standardize the core capabilities and deployment model needed for interoperable workloads.
2. Suppliers can collaborate on additional placement metadata and matching rules through a generic, specification-defined mechanism.

This is particularly useful for scenarios where suppliers intentionally target only a subset of Margo devices, such as:

* virtual machine images deployed only to devices that support a specific hypervisor
* WASM artifacts deployed only to devices that support a specific runtime and package format
* opaque gateways that must choose a compatible downstream device even when the WFM cannot address that device directly

## Requirements alignment acknowledgement

This proposal aligns with the Margo goal of defining interoperable interfaces while avoiding unnecessary standardization of vendor-specific implementation details.

It directly supports the use cases behind [issue 93](https://github.com/margo/specification/issues/93) and applies to the following existing specification surfaces:

* [Device Capabilities](https://docs.margo.org/specification/margo-management-interface/device-capabilities)
* [Application Description](https://docs.margo.org/specification/applications/application-description)
* [Application Deployment / Desired State](https://specification.margo.org/margo-api-reference/workload-api/desired-state-api/desired-state/)

This SUP is intentionally limited in scope.

The following items are out of scope:

* standardizing any new runtime, package format, or execution environment
* defining a global registry of approved supplier labels
* defining gateway inventory models beyond the reuse of `deviceConstraints` in `ApplicationDeployment`
* defining scheduling policies such as priorities, scoring, or anti-affinity

### SUP dependency

This SUP has a dependency on the SUP to [move device roles to capabilities](https://github.com/margo/specification-enhancements/pull/50). The example device capabilities in this SUP are based on what was proposed at the time and not a reflection of changes to the device capabilities beyond the introduction of `labels`. This SUP assumes compliance with whatever the final approved decision is on the SUP to remove device roles from the device capabilities document.

## Technical proposal

### Overview

This SUP introduces three normative changes:

1. add an optional top-level `labels` object to `DeviceCapabilitiesManifest`
2. replace the `requiredResources` field with the `deviceConstraints` field for each deployment profile in `ApplicationDescription`
3. add the `deviceConstraints` field to each deployment profile in `ApplicationDeployment` so an opaque gateway can perform device selection locally when the WFM cannot target a specific downstream device

The mechanism is generic by design:

* `properties` remain Margo-defined fields with Margo-defined semantics
* `labels` are supplier-defined key/value metadata with no semantics implied by Margo beyond exact matching rules
* `deviceConstraints` separates minimum capacity requirements from eligibility matching rules evaluated against `properties` and `labels`

### 1. Device Capabilities changes

The `DeviceCapabilitiesManifest` document is extended with an optional top-level `labels` field.

#### Schema addition

```json
{
    "apiVersion": "device.margo.org/v1alpha1",
    "kind": "DeviceCapabilitiesManifest",
    "properties": {... existing margo-defined properties ...},
    "labels": {
      "<label-key>": <value (number, string, boolean, or array of these)>  
    }
}
```

#### Semantics

* `properties` remain the canonical location for all Margo-standardized device fields.
* `labels` are optional and MAY be omitted.
* A label key is case-sensitive.
* A label value MUST be either:
  * a string, number, or boolean
  * an array of one or more strings, or numbers
* Margo does not assign semantics to any particular label key or value.
* Suppliers MAY define and document shared label vocabularies outside the Margo specification.
* Implementations SHOULD use stable, collision-resistant label names. Prefixing with an organization domain is RECOMMENDED for supplier-specific labels.

Examples:

```json
{
  "labels": {
    "example.com/hypervisor": "hyper-v",
    "example.com/wasm.runtime": ["wamr"],
    "example.com/wasm.package.format": [".wasm", ".aot"],
    "example.com/os": "zephyr"
  }
}
```

#### Add "Custom" runtime and deploymentType options

* The `supportedRuntimes` array is updated to allow for "oci" and "custom". Using "custom" indicates the device has a runtime that is not officially supported by Margo.
* The `supportedDeploymentTypes` array is updated to allow for "compose", "helm", and "custom". Using "custom" indicates the device is capable of deploying applications using a deployment type that is not officially supported by Margo.

> **Note:** This is based on the current proposal to [move device roles to capabilities](https://github.com/margo/specification-enhancements/pull/50). Any changes made to that proposal before it is approved may impact this section, so the intention is for this to be compatible with what is approved for that SUP.

### 2. Device constraints model

Each deployment profile in `ApplicationDescription` and `ApplicationDeployment` MUST define a `deviceConstraints` field.

#### Schema

```yaml
deviceConstraints:
  capacityRequirements:
    cpu:
      cores: 1.5
      architectures: ["amd64"]
    memory: 1024Mi
    storage: 10Gi
  eligibilityRules:
    - propertySelector:
        matchExpressions:
          - key: /vendor
            operator: In
            values: ["Example Vendor"]
      labelSelector:
        matchExpressions:
          - key: example.com/hypervisor
            operator: In
            values: ["hyper-v"]
```

`deviceConstraints` supports:

* `capacityRequirements` - required minimum CPU, memory, and storage requirements for the deployment profile
* `eligibilityRules` - optional rule terms evaluated against `DeviceCapabilitiesManifest.properties` and `DeviceCapabilitiesManifest.labels`

Each entry in the `eligibilityRules` array is a `DeviceEligibilityRule`.

`DeviceEligibilityRule` supports:

* `propertySelector` - matches against Margo-defined fields under `DeviceCapabilitiesManifest.properties`
* `labelSelector` - matches against supplier-defined fields under `DeviceCapabilitiesManifest.labels`

Each selector contains a `matchExpressions` array.

Each expression has the following schema:

```yaml
- key: <string>
  operator: In | NotIn | Exists | DoesNotExist | Gt | Lt | ContainsAll | ContainsAny
  values: [<object>, ...] # not required for Exists, DoesNotExist, ContainsAll, ContainsAny
  itemSelector: # required for ContainsAll, ContainsAny
```

#### Property selector key format

`propertySelector.matchExpressions[].key` MUST be a JSON Pointer as defined by [RFC 6901](https://datatracker.ietf.org/doc/html/rfc6901), evaluated relative to the `properties` object.

Examples:

* `/vendor`
* `/id`
* `/modelNumber`
* `/capabilities/supportedDeploymentTypes`

This version of the SUP only defines selector behavior for property values that resolve to either:

* a string, number, or boolean
* an array of strings, or numbers
* an array of objects

If a property selector resolves to any other JSON type, the expression MUST evaluate to `false`.

If a property selector resolve to an array of objects, `ContainsAll` or `ContainsAny` must be used or else the expression MUST evaluate to `false`.

#### Label selector key format

`labelSelector.matchExpressions[].key` MUST be the exact label key to evaluate within `labels`.

This version of the SUP only defines selector behavior for label values that resolve to either:

* a string, number, or boolean
* an array of strings, or numbers

If a label selector resolves to any other JSON type, the expression MUST evaluate to `false`.

#### Capacity requirements semantics

The `capacityRequirements` rules are:

* `cpu.cores` is the minimum number of CPU cores required on the target device.
* `cpu.architectures`, when present, restricts acceptable CPU architectures.
* `memory` is the minimum memory required by the deployment profile.
* `storage` is the minimum storage required by the deployment profile.
* A device MUST satisfy all specified `capacityRequirements` to remain eligible for further evaluation.

#### Eligibility rules matching semantics

The matching rules are:

* All `matchExpressions` within a single selector are combined using logical AND.
* If a `DeviceEligibilityRule` contains both `propertySelector` and `labelSelector`, both selectors must match for that rule to match.
* The `eligibilityRules` array is combined using logical OR. A device matches eligibility rules if at least one `DeviceEligibilityRule` matches.
* If `eligibilityRules` is omitted, the deployment profile has no additional selector-based constraint beyond `capacityRequirements`.
* If both `capacityRequirements` and `eligibilityRules` are present, implementations MUST evaluate `capacityRequirements` first and MUST only evaluate `eligibilityRules` for devices that satisfy the required capacity.

Operator behavior:

* `In`: true when the referenced value equals one of `values`, or when the referenced array contains at least one element that equals one of `values`
* `NotIn`: true when the referenced key exists and none of its values match any entry in `values`
* `Exists`: true when the referenced key is present
* `DoesNotExist`: true when the referenced key is absent
* `Gt`: true when the referenced value is greater than `values`.
* `Lt`: true when the referenced value is less than `values`.
* `ContainsAll`: True when the referenced value is an array of objects and at least one array element satisfies all `itemSelector.matchExpressions` (AND logic).
* `ContainsAny`: True when the referenced value is an array of objects and at least one array element satisfies any `itemSelector.matchExpression` (OR logic).

String comparisons MUST be exact and case-sensitive.

For `In` and `NotIn`:

* `values` MUST be present
* `values` MUST contain one or more strings or numbers, or one boolean
* `values` MUST be the same data type when indicating more than one.

For `Exists` and `DoesNotExist`:

* `values` MUST be omitted

For `Gt` and `Lt`:

* `values` MUST be present
* `values` MUST be parsable as numbers
* `values` MUST only contain a single number

For `ContainsAll` and `ContainsAny`:

* `itemSelector` MUST be present
* `itemSelector.matchExpressions` MUST contain one or more expressions
* `values` MUST be omitted
* Keys within `itemSelector.matchExpressions` are JSON Pointers relative to each array element (not absolute)
* Expressions within `itemSelector.matchExpressions` are combined using `AND` logic for `ContainsAll` and `OR` logic for `ContainsAny`

#### Object Array Item Matching

`ContainsAll` and `ContainsAny` operators enable matching on properties of objects within an array. These operators are used when the property referenced by `key` resolves to an array of objects.

**`ContainsAll` (array element with all conditions):**

Evaluates to true when at least one array element satisfies all conditions specified in `itemSelector.matchExpressions`. All expressions within the `itemSelector` are combined using AND logic.

Example: Match a device that has at least one GPU peripheral manufactured by NVIDIA:

```yaml
propertySelector:
  matchExpressions:
    - key: /resources/peripherals
      operator: ContainsAll
      itemSelector:
        matchExpressions:
          - key: /type
            operator: In
            values: ["gpu"]
          - key: /manufacturer
            operator: In
            values: ["NVIDIA"]
```

**`ContainsAny` (array element with any condition):**

Evaluates to true when at least one array element satisfies any condition specified in `itemSelector.matchExpressions`. Expressions within the `itemSelector` are combined using OR logic.

Example: Match a device that has at least one peripheral that is either a GPU or a high-speed NIC:

```yaml
propertySelector:
  matchExpressions:
    - key: /resources/peripherals
      operator: ContainsAny
      itemSelector:
        matchExpressions:
          - key: /type
            operator: In
            values: ["gpu", "nic"]
```

**Interaction with other expressions:**

Multiple `matchExpressions` within the same `propertySelector` remain AND'd together. For example:

```yaml
propertySelector:
  matchExpressions:
    - key: /vendor
      operator: In
      values: ["Vendor Name"]
    - key: /resources/peripherals
      operator: ContainsAll
      itemSelector:
        matchExpressions:
          - key: /type
            operator: In
            values: ["gpu"]
```

This evaluates to true only when the device vendor matches **AND** the device has at least one GPU peripheral.
  
### 3. Application Description changes

Each deployment profile type in `ApplicationDescription` is extended with the `deviceConstraints` field using the schema and semantics defined above.

This field allows an application supplier to declare the minimum device characteristics required for that deployment profile.

Example:

```yaml
apiVersion: margo.org/v1-alpha1
kind: ApplicationDescription
metadata:
  id: com.appforge-dynamics.sys-sec-mon
  name: System and Security Monitoring
  version: "1.0"
deploymentProfiles:
  - type: custom
    id: com.appforge-dynamics.sys-sec-mon.hyperv
    components:
      - name: sys-sec-mon
        properties:
          repository: oci://appforge-dynamics.azurecr.io/hyperv/sys-sec-mon
          revision: 1.0.0
    deviceConstraints:
      capacityRequirements:
        cpu:
          cores: 1
        memory: 1024Mi
        storage: 5Gi
      eligibilityRules:
        - propertySelector:
            matchExpressions:
              - key: /vendor
                operator: In
                values: ["EdgeCircuit Systems", "NanoEdge Devices"]
          labelSelector:
            matchExpressions:
              - key: example.com/hypervisor
                operator: In
                values: ["hyper-v"]
```

Normative behavior:

* A WFM MUST evaluate `deviceConstraints` before selecting a target device for a deployment profile.
* A device that does not satisfy the minimum required CPU, memory, and storage defined in the `capacityRequirements` MUST NOT be selected for that profile.
* If `eligibilityRules` are present, a device that does not satisfy the required rule evaluation MUST NOT be selected for that profile.
* If no available device satisfies the profile's required constraints, the deployment MUST be reported as not placeable according to the implementation's existing status model.

#### Supporting "custom" deployment types

If the application is packaged using something other than the officially supported deployment types of "helm" or "compose" the deployment type is set to "custom" to indicate the application package is using an unofficial deployment type. When using the "custom" deployment type, there MUST be a `deviceConstraints` rule.

### 4. Application Deployment changes

Each deployment profile in `ApplicationDeployment.spec.deploymentProfiles[]` is extended with the same optional `deviceConstraints` field.

This field is primarily required for opaque gateway scenarios where the WFM can target only the gateway, while the gateway itself must choose the final downstream device.

Normative behavior:

* For non-gateway and see-through gateway scenarios, `ApplicationDeployment` MAY omit `deviceConstraints` if the selected target device is already known by the WFM.
* For opaque gateway scenarios, when device selection must occur behind the gateway, the `ApplicationDeployment` sent to the gateway MUST include the required `deviceConstraints` for the selected deployment profile.
* An opaque gateway that receives `deviceConstraints` in `ApplicationDeployment` MUST evaluate them against the capabilities of its downstream devices before placing the workload.
* An opaque gateway MUST NOT place the workload on a downstream device that fails the constraints evaluation.
* When the `ApplicationDeployment` is derived from an `ApplicationDescription`, the included `deviceConstraints` MUST be identical to or more restrictive than the source deployment profile. It MUST NOT broaden the set of eligible devices.

Example:

```yaml
apiVersion: application.margo.org/v1alpha1
kind: ApplicationDeployment
metadata:
  annotations:
    applicationId: com.appforge-dynamics.sys-sec-mon
    id: a3e2f5dc-912e-494f-8395-52cf3769bc06
  name: com.appforge-dynamics.sys-sec-mon-deployment
  namespace: appforge-dynamics
spec:
  deploymentProfiles:
    - type: custom
      id: com.appforge-dynamics.sys-sec-mon.hyperv
      components:
        - name: sys-sec-mon
          properties:
            repository: oci://appforge-dynamics.azurecr.io/hyperv/sys-sec-mon
            revision: 1.0.0
      deviceConstraints:
        capacityRequirements:
          cpu:
            cores: 1
          memory: 1024Mi
          storage: 5Gi
        eligibilityRules:
          - labelSelector:
              matchExpressions:
                - key: example.com/hypervisor
                  operator: In
                  values: ["hyper-v"]
```

### 5. Example use cases

#### Custom deployment of Hyper-V virtual machines via Margo

AppForge Dynamics is a company that builds security and monitoring applications. Their applications are Windows-based and can be deployed via virtual machine images that target Hyper-V.

AppForge Dynamics has partnered with two companies, EdgeCircuit Systems and NanoEdge Devices, that will supply Windows servers that can be used to deploy AppForge Dynamics virtual machines.

AppForge Dynamics has agreed to follow the Margo application package approach to package their virtual images inside an OCI blob and use the Margo Application description to make their application available. EdgeCircuit Systems and NanoEdge Devices both have Windows servers that are running their own implementation of the Margo WFM client. While they cannot deploy applications targeting Kubernetes or Compose, they can deploy AppForge Dynamics's apps.

It should be possible for these three vendors to collaborate and deploy these virtual machines to the targeted devices supplied by EdgeCircuit Systems and NanoEdge Devices. They should be able to do this using implementations based on the Margo specification while using a WFM that knows nothing about what these three suppliers are doing. There are no expectations that these VMs will be deployed on any other Margo-conformant devices except those provided by these two device suppliers. There are no expectations that these devices will be able to deploy anything but these VMs.

##### Application Description

```yaml
apiVersion: margo.org/v1-alpha1
kind: ApplicationDescription
metadata:
  id: com.appforge-dynamics.sys-sec-mon
  name: System and Security Monitoring
  description: System and Security Monitoring Application for Windows
  version: "1.0"
  catalog:
    application:
      icon: ./resources/logo.png
      tagline: Intuitive system and security monitoring.
      descriptionFile: ./resources/description.md
      releaseNotes: ./resources/release-notes.md
      licenseFile: ./resources/license.pdf
      site: http://appforge-dynamics.com/monitoring
      tags: ["monitoring", "Security", "Hyper-V", "Windows"]
    organization:
      - name: AppForge Dynamics
        site: http://appforge-dynamics.com
deploymentProfiles:
  - type: custom
    id: com.appforge-dynamics.sys-sec-mon.hyperv
    components:
      - name: sys-sec-mon
        properties:  
          repository: oci://apppforge-dynamics.azurecr.io/hyperv/sys-sec-mon
          revision: 1.0.0
    deviceConstraints:
      capacityRequirements:
        cpu:
          cores: 1
        memory: 1024Mi
        storage: 5Gi
      eligibilityRules:
        - propertySelector:
            matchExpressions:
            - key: /vendor
              operator: In
              values:
              - "EdgeCircuit Systems"
              - "NanoEdge Devices"
          labelSelector:
            matchExpressions:
            - key: example.com/hyper-v.host
              operator: In
              values:
              - true
parameters: ...
configuration: ...
```

##### Device Capabilities

```json
{
    "apiVersion": "device.margo.org/v1alpha1",
    "kind": "DeviceCapabilitiesManifest",
    "properties": {
        "id": "com.edge-circuit-systems.hardware.G12",
        "vendor": "EdgeCircuit Systems",
        "modelNumber": "EF1.234.32",
        "serialNumber": "SN12928342125",
        "memory": "64 Gi",
        "storage": "1862 Gi",
        "peripherals": [],
        "interfaces": [],
        "otelCollector": true,
        "supportedRuntimes": ["custom"],
        "supportedDeploymentTypes": ["custom"]
    },
    "labels": {
      "example.com/hyper-v.host": true 
    }
}
```

#### Custom native-wasm deployment to Zephyr devices via Margo

A conglomerate of three application suppliers (MicroKil Technologies, ForgeFlux Systems, Wasmotive Industrial) and three device suppliers (AeroByte Microsystems, Northforge Embedded Systems, SilicaTrail Technologies) has collaborated to come up with an agreement on how to use Margo to enable application vendors to deploy WASM based applications to embedded and IoT devices running Zephyr RTOS. They have agreed that the most compatible WASM runtime for these devices right now is WAMR, but this could change in the future.

Based on feedback from customers, they feel being conformant with Margo is the best way to meet their needs. Since Margo does not officially support native wasm deployment types, they need a way to match up their applications to suitable target devices.

> **Note:** Currently native-wasm isn't officially supported by the Margo specification. This is just an example of how it could be described until such time as native-wasm is officially supported.

They have agreed that they want their customers to be able to use any Margo conformant workload fleet manager to deploy their WASM based applications to these special devices. In order to accomplish this, the application vendors have agreed to package their applications using Margo's application package format and distribute their app artifacts using OCI blobs. The hardware vendors have agreed to implement some of the workload fleet manager client spec to enable onboarding and application deployment, but not everything (e.g, no OTEL collector).

Working together, the group has identified a set of labels they want to use to enable the three app vendors to identify the devices from the three supporting vendors. They've used an approach they feel gives them enough flexibility to meet everyone's needs while allowing for support for other wasm runtimes, wasm/wasi versions and package formats later.

The group has also agreed to publish information about the agreed-on labels so other application and device suppliers can make use of them if they wish.

##### Application Description

```yaml
apiVersion: margo.org/v1-alpha1
kind: ApplicationDescription
metadata:
  id: com.wasmotive-industrial.asset-tracking
  name: Wasmotive Smart Asset Tracking
  description: Smart Asset Tracking
  version: "2026.2"
  catalog:
    application:
      icon: ./resources/logo.png
      tagline: Track those assets.
      descriptionFile: ./resources/description.md
      releaseNotes: ./resources/release-notes.md
      licenseFile: ./resources/license.pdf
      site: http://wasmotive-industrial.com/embedded
      tags: ["embedded", "real-time", "tracking"]
    organization:
      - name: Wasmotive Industrial
        site: http://wasmotive-industrial.com
deploymentProfiles:
  - type: custom
    id: com.wasmotive-industrial.asset-tracking.2026.2
    components:
      - name: asset-tracking
        properties:  
          repository: oci://com.wasmotive-industrial.azurecr.io/apps/asset-tracking-2026
          revision: 2026.2.13
    deviceConstraints:
      capacityRequirements:
        cpu:
          cores: 0.5
          architectures: ["arm64"]
        memory: 256Mi
        storage: 1Gi
      eligibilityRules:
        - labelSelector:
            matchExpressions:
            - key: example.com/os
              operator: In
              values:
              - "Zephyr RTOS"
            - key: example.com/wasm.runtime
              operator: In
              values:
              - "WAMR"
            - key: example.com/wasm.package.format
              operator: In
              values:
              - ".aot"
            - key: example.com/wasi.version
              operator: In
              values:
              - "Snapshot Preview 1"
            - key: example.com/wasm.version
              operator: In
              values:
              - "MVP"
parameters: ...
configuration: ...
```

##### Device Capabilities

```json
{
    "apiVersion": "device.margo.org/v1alpha1",
    "kind": "DeviceCapabilitiesManifest",
    "properties": {
        "id": "com.silica-trail-tech.ebd18",
        "vendor": "SilicaTrail Technologies",
        "modelNumber": "TRS-002",
        "serialNumber": "SN00008983",
        "memory": "64 Gi",
        "storage": "1862 Gi",
        "peripherals": [],
        "interfaces": [],
        "otelCollector": true,
        "supportedRuntimes": ["custom"],
        "supportedDeploymentTypes": ["custom"]
    },
    "labels": {
      "example.com/wasm.runtime": "WAMR",
      "example.com/wasm.version": ["MVP", "MVP+"],
      "example.com/wasi.version": ["Snapshot Preview 1"],
      "example.com/wasm.package.format": [".wasm", ".aot", ".o", ".a"],
      "example.com/os": "Zephyr RTOS"
    }
}
```

#### Custom deployment of Hyper-V virtual machines via Margo behind opaque gateway

For this use case we have an opaque gateway setup that has three devices behind it. One device supports the compose deployment type, one device supports the helm deployment type, and the third device supports the custom Hyper-V deployment type from the use case above.

##### Application Description

###### Application description for hyper-v VM deployment

```yaml
apiVersion: margo.org/v1-alpha1
kind: ApplicationDescription
metadata:
  id: com.appforge-dynamics.sys-sec-mon
  name: System and Security Monitoring
  description: System and Security Monitoring Application for Windows
  version: "1.0"
  catalog:
    application:
      icon: ./resources/logo.png
      tagline: Intuitive system and security monitoring.
      descriptionFile: ./resources/description.md
      releaseNotes: ./resources/release-notes.md
      licenseFile: ./resources/license.pdf
      site: http://appforge-dynamics.com/monitoring
      tags: ["monitoring", "Security", "Hyper-V", "Windows"]
    organization:
      - name: AppForge Dynamics
        site: http://appforge-dynamics.com
deploymentProfiles:
  - type: custom
    id: com.appforge-dynamics.sys-sec-mon.hyperv
    components:
      - name: sys-sec-mon
        properties:  
          repository: oci://apppforge-dynamics.azurecr.io/hyperv/sys-sec-mon
          revision: 1.0.0
    deviceConstraints:
      capacityRequirements:
        cpu:
          cores: 1
        memory: 1024Mi
        storage: 5Gi
      eligibilityRules:
        - labelSelector:
            matchExpressions:
            - key: example.com/hyper-v.host
              operator: In
              values:
              - true
parameters: ...
configuration: ...
```

###### Application description for helm deployment

```yaml
apiVersion: margo.org/v1-alpha1
kind: ApplicationDescription
metadata:
  id: com.leaf-industrial.mastercontrol
  name: Master Control
  description: Master Control
  version: "9.3"
  catalog:
    application:
      icon: ./resources/logo.png
      tagline: Intuitive system and security monitoring.
      descriptionFile: ./resources/description.md
      releaseNotes: ./resources/release-notes.md
      licenseFile: ./resources/license.pdf
      site: http://leaf-industrial.com/monitoring
      tags: ["Control Systems"]
    organization:
      - name: Leaf Industrial
        site: http://Leaf Industrial.com
deploymentProfiles:
  - type: helm
    id: com.leaf-industrial.AGA.mastercontrol
    components:
      - name: mastercontrol
        properties:  
          repository: oci://leaf-industrial.azurecr.io/AGA/charts/mastercontrol
          revision: 9.3.324
    deviceConstraints:
      capacityRequirements:
        cpu:
          cores: 1
        memory: 1024Mi
        storage: 5Gi
parameters: ...
configuration: ...
```

###### Application description for compose deployment

```yaml
apiVersion: margo.org/v1-alpha1
kind: ApplicationDescription
metadata:
  id: com.leaf-industrial.masterreporting
  name: Master Reporting
  description: Master Reporting
  version: "6.5"
  catalog:
    application:
      icon: ./resources/logo.png
      tagline: System Reporting.
      descriptionFile: ./resources/description.md
      releaseNotes: ./resources/release-notes.md
      licenseFile: ./resources/license.pdf
      site: http://leaf-industrial.com
      tags: ["Control Systems", "Reporting"]
    organization:
      - name: Leaf Industrial
        site: http://Leaf Industrial.com
deploymentProfiles:
  - type: compose
    id: com.leaf-industrial.PMC.masterreporting
    components:
      - name: masterreporting
        properties:  
          repository: oci://leaf-industrial.azurecr.io/PMC/compose/masterreporting
          revision: 6.5.4
    deviceConstraints:
      capacityRequirements:
        cpu:
          cores: 1
        memory: 1024Mi
        storage: 5Gi
parameters: ...
configuration: ...
```

##### Device Capabilities

The device capabilities has the combination of all three hardware devices

```json
{
    "apiVersion": "device.margo.org/v1alpha1",
    "kind": "DeviceCapabilitiesManifest",
    "properties": {
        "id": "com.pearware.gateway",
        "vendor": "PearWare",
        "modelNumber": "GT45",
        "serialNumber": "SN7383221",
        "memory": "64 Gi",
        "storage": "1862 Gi",
        "peripherals": [],
        "interfaces": [],
        "otelCollector": true,
        "supportedRuntimes": ["oci","custom"],
        "supportedDeploymentTypes": ["helm", "compose", "custom"]
    },
    "labels": {
      "example.com/hyper-v.host": true,
    }
}
```

#### Application Deployment

##### Application deployment for Hyper-V

Since customer only sees the Gateway, and not the devices behind it, the Application Deployment needs to include the required device constraints so the Gateway knows what device to match the deployment to.

```yaml
apiVersion: application.margo.org/v1alpha1
kind: ApplicationDeployment
metadata:
    annotations:
        applicationId: com.appforge-dynamics.sys-sec-mon
        id: a3e2f5dc-912e-494f-8395-52cf3769bc06
    name: com.appforge-dynamics.sys-sec-mon-deployment
    namespace: appforge-dynamics
spec:
  deploymentProfiles:
    - type: custom
      id: com.appforge-dynamics.sys-sec-mon.hyperv
      components:
        - name: sys-sec-mon
          properties:  
            repository: oci://apppforge-dynamics.azurecr.io/hyperv/sys-sec-mon
            revision: 1.0.0
      deviceConstraints:
        capacityRequirements:
          cpu:
            cores: 1
          memory: 1024Mi
          storage: 5Gi
        eligibilityRules:
          - labelSelector:
              matchExpressions:
              - key: example.com/hyper-v.host
                operator: In
                values:
                - true
      parameters: ...
```

##### Application deployment for Helm

```yaml
apiVersion: application.margo.org/v1alpha1
kind: ApplicationDeployment
metadata:
    annotations:
        applicationId: com.leaf-industrial.mastercontrol
        id: 38ea2934-539f-e271-3872-263883be7270
    name: com.leaf-industrial.mastercontrol-deployment
    namespace: leaf-industrial
spec:
  deploymentProfiles:
    - type: helm
      id: com.leaf-industrial.AGA.mastercontrol
      components:
        - name: mastercontrol
          properties:  
            repository: oci://leaf-industrial.azurecr.io/AGA/charts/mastercontrol
            revision: 9.3.324
      deviceConstraints:
        capacityRequirements:
          cpu:
            cores: 1
          memory: 1024Mi
          storage: 5Gi
      parameters: ...
```

### 6. Conformance expectations

To claim conformance with this SUP:

* a producer of `DeviceCapabilitiesManifest` MUST preserve the existing `properties` structure and MAY include `labels`
* a consumer implementing placement decisions MUST correctly evaluate `deviceConstraints` using the rules defined above
* a WFM or gateway MUST treat `labels` as opaque supplier-defined metadata and MUST NOT infer any additional semantics beyond exact selector evaluation
* an implementation MUST ignore unknown label keys unless they are referenced by a selector

## Alternatives considered (optional)

### Add dedicated capability fields for each new runtime or packaging model

This approach was rejected because it would continuously expand the core specification with supplier-specific concepts that Margo cannot reasonably standardize. It also creates pressure to define semantics for proprietary or niche runtimes that are intentionally outside Margo's scope.

### Add only custom runtime fields and no generic constraints model

This approach was rejected because the underlying problem is broader than runtimes. Suppliers may need to target devices based on operating system, gateway topology, hypervisor presence, package format, vendor collaboration, or other non-standard characteristics. A generic constraints model solves the broader problem once.

### Restrict matching to labels only

This approach was rejected because some useful constraints already exist as standardized device properties, such as vendor identity or supported deployment types. Reusing those properties avoids duplication and keeps Margo-defined fields authoritative.

### Alternative SUP focused on custom or specific runtimes

This SUP is intentionally a more general alternative to proposals that solve issue 93 by adding runtime-specific structures. Its advantage is that it provides a single placement mechanism that works for custom runtimes, heterogeneous gateway topologies, and future supplier-defined constraints without reopening the specification for each new case.

### Limiting selectors to simple types only

During development, it was recognized that `propertySelector` initially only supported matching on simple types (strings, numbers, booleans, and arrays thereof), which excluded matching on complex types such as the `peripherals` array of objects.

This limitation was addressed by introducing `ContainsAll` and `ContainsAny` operators with `itemSelector` support. These operators enable recursive selector logic that matches based on properties of objects within arrays, providing a general solution for matching on complex device properties without requiring a custom query language or expanding the set of supported types indefinitely.

This approach maintains the clarity and safety of the selector pattern while extending its expressiveness to handle real-world device capability structures like peripherals, network interfaces, and other composite properties.

## Rejection reason

None. This SUP is proposed for consideration.