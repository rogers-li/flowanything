package config

import "fmt"

func CheckRuntimeCompatibility(bundle BundleSpec, manifest RuntimeManifest) error {
	var errs ValidationErrors
	if manifest.RuntimeID == "" {
		errs.Add("runtime_id", "runtime id is required")
	}
	if manifest.Target == "" {
		errs.Add("target", "runtime target is required")
	}
	if len(bundle.Runtime.Targets) > 0 && !targetAllowed(bundle.Runtime.Targets, manifest.Target) {
		errs.Add("target", fmt.Sprintf("runtime target %q is not allowed by bundle", manifest.Target))
	}
	supported := map[string]CapabilitySupport{}
	for _, capability := range manifest.Capabilities {
		if capability.Name == "" {
			continue
		}
		supported[capability.Name] = capability
	}
	checkCapabilities(&errs, "runtime.required_capabilities", bundle.Runtime.RequiredCapabilities, supported)
	checkResourceCapabilities(&errs, bundle, supported)
	if errs.HasErrors() {
		return errs
	}
	return nil
}

func targetAllowed(allowed []RuntimeTarget, actual RuntimeTarget) bool {
	for _, target := range allowed {
		if target == actual {
			return true
		}
		if target == RuntimeMobile && (actual == RuntimeIOS || actual == RuntimeAndroid) {
			return true
		}
	}
	return false
}

func checkResourceCapabilities(errs *ValidationErrors, bundle BundleSpec, supported map[string]CapabilitySupport) {
	for _, agent := range bundle.Resources.Agents {
		if agent.Disabled {
			continue
		}
		checkRuntimeRequirement(errs, "resources.agents."+agent.ID+".runtime", agent.Runtime, supported)
	}
	for _, skill := range bundle.Resources.Skills {
		if skill.Disabled {
			continue
		}
		checkRuntimeRequirement(errs, "resources.skills."+skill.ID+".runtime", skill.Runtime, supported)
	}
	for _, tool := range bundle.Resources.Tools {
		if tool.Disabled {
			continue
		}
		checkRuntimeRequirement(errs, "resources.tools."+tool.ID+".runtime", tool.Runtime, supported)
	}
	for _, workflow := range bundle.Resources.Workflows {
		if workflow.Disabled {
			continue
		}
		checkRuntimeRequirement(errs, "resources.workflows."+workflow.ID+".runtime", workflow.Runtime, supported)
	}
	for _, connector := range bundle.Resources.Connectors {
		if connector.Disabled {
			continue
		}
		checkRuntimeRequirement(errs, "resources.connectors."+connector.ID+".runtime", connector.Runtime, supported)
	}
	for _, model := range bundle.Resources.Models {
		if model.Disabled {
			continue
		}
		checkRuntimeRequirement(errs, "resources.models."+model.ID+".runtime", model.Runtime, supported)
	}
	for _, knowledge := range bundle.Resources.KnowledgeBases {
		if knowledge.Disabled {
			continue
		}
		checkRuntimeRequirement(errs, "resources.knowledge_bases."+knowledge.ID+".runtime", knowledge.Runtime, supported)
	}
}

func checkRuntimeRequirement(errs *ValidationErrors, path string, runtime RuntimeRequirementSpec, supported map[string]CapabilitySupport) {
	requireBuiltIn(errs, path, supported, runtime.Network, "network")
	requireBuiltIn(errs, path, supported, runtime.FileRead, "file.read")
	requireBuiltIn(errs, path, supported, runtime.FileWrite, "file.write")
	requireBuiltIn(errs, path, supported, runtime.Location, "location")
	requireBuiltIn(errs, path, supported, runtime.Camera, "camera")
	requireBuiltIn(errs, path, supported, runtime.Microphone, "microphone")
	requireBuiltIn(errs, path, supported, runtime.ServerProxyAllowed, "server.proxy")
	for _, secret := range runtime.Secrets {
		if secret.Required {
			requireBuiltIn(errs, path, supported, true, "secret")
		}
	}
	checkCapabilities(errs, path+".capabilities", runtime.Capabilities, supported)
}

func requireBuiltIn(errs *ValidationErrors, path string, supported map[string]CapabilitySupport, required bool, capability string) {
	if !required {
		return
	}
	if _, ok := supported[capability]; !ok {
		errs.Add(path, fmt.Sprintf("runtime capability %q is required", capability))
	}
}

func checkCapabilities(errs *ValidationErrors, path string, required []CapabilityRequirement, supported map[string]CapabilitySupport) {
	for i, capability := range required {
		if capability.Name == "" || !capability.Required {
			continue
		}
		if _, ok := supported[capability.Name]; !ok {
			errs.Add(fmt.Sprintf("%s[%d]", path, i), fmt.Sprintf("runtime capability %q is required", capability.Name))
		}
	}
}
