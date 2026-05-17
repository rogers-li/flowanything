package prompt

import (
	"fmt"
	"strings"

	"flow-anything/core/capability"
	"flow-anything/core/schema"
)

func FromCapabilityDescriptor(item capability.Descriptor) CapabilityDescriptor {
	return CapabilityDescriptor{
		ID:           item.ID,
		Kind:         string(item.Kind),
		Name:         item.Name,
		Description:  item.Description,
		InputSchema:  item.InputSchema,
		OutputSchema: item.OutputSchema,
	}
}

func FromCapabilityDescriptors(items []capability.Descriptor) []CapabilityDescriptor {
	out := make([]CapabilityDescriptor, 0, len(items))
	for _, item := range items {
		out = append(out, FromCapabilityDescriptor(item))
	}
	return out
}

func DescribeCapabilities(capabilities []CapabilityDescriptor) string {
	if len(capabilities) == 0 {
		return "(no capabilities available)"
	}
	var builder strings.Builder
	for _, item := range capabilities {
		builder.WriteString(fmt.Sprintf("- type=%s; id=%s; name=%s; description=%s\n", item.Kind, item.ID, item.Name, item.Description))
		if len(item.InputSchema) > 0 {
			builder.WriteString("  input:\n")
			writeIndented(&builder, schema.Describe(item.InputSchema), "    ")
			builder.WriteString("\n")
		}
		if len(item.OutputSchema) > 0 {
			builder.WriteString("  output:\n")
			writeIndented(&builder, schema.Describe(item.OutputSchema), "    ")
			builder.WriteString("\n")
		}
	}
	return strings.TrimRight(builder.String(), "\n")
}

func writeIndented(builder *strings.Builder, text, prefix string) {
	for _, line := range strings.Split(text, "\n") {
		builder.WriteString(prefix)
		builder.WriteString(line)
		builder.WriteString("\n")
	}
}
