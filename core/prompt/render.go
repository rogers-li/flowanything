package prompt

import (
	"fmt"
	"sort"
	"strings"
)

func BuildText(spec Spec) string {
	parts := []string{}
	if strings.TrimSpace(spec.System) != "" {
		parts = append(parts, strings.TrimSpace(spec.System))
	}
	if strings.TrimSpace(spec.Developer) != "" {
		parts = append(parts, "Developer Instructions:\n"+strings.TrimSpace(spec.Developer))
	}
	return strings.Join(parts, "\n\n")
}

// RenderTemplate performs simple {{var}} replacement. It is intentionally
// deterministic and side-effect free; advanced template engines should live in
// adapters until the config protocol explicitly supports them.
func RenderTemplate(template string, variables map[string]any) string {
	keys := make([]string, 0, len(variables))
	for key := range variables {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := template
	for _, key := range keys {
		out = strings.ReplaceAll(out, "{{"+key+"}}", fmt.Sprint(variables[key]))
	}
	return out
}
