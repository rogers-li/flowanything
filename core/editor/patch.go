package editor

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"flow-anything/core/config"
)

// ApplyPatch applies JSON-pointer patch operations to a bundle draft and
// returns the edited bundle. It does not validate automatically so callers can
// preserve partially edited drafts; call InspectDraft or config.ValidateBundle
// when validation is needed.
func ApplyPatch(bundle config.BundleSpec, patch PatchSet) (config.BundleSpec, error) {
	var document any
	data, err := json.Marshal(bundle)
	if err != nil {
		return config.BundleSpec{}, err
	}
	if err := json.Unmarshal(data, &document); err != nil {
		return config.BundleSpec{}, err
	}
	for _, operation := range patch.Operations {
		if err := applyPatchOperation(&document, operation); err != nil {
			return config.BundleSpec{}, err
		}
	}
	encoded, err := json.Marshal(document)
	if err != nil {
		return config.BundleSpec{}, err
	}
	var out config.BundleSpec
	if err := json.Unmarshal(encoded, &out); err != nil {
		return config.BundleSpec{}, err
	}
	return out, nil
}

// DiffBundles returns a compact patch set that transforms before into after.
// Map fields are diffed recursively; arrays are replaced as a unit to keep the
// operation deterministic and simple for canvas drafts.
func DiffBundles(before, after config.BundleSpec) (PatchSet, error) {
	beforeValue, err := toJSONValue(before)
	if err != nil {
		return PatchSet{}, err
	}
	afterValue, err := toJSONValue(after)
	if err != nil {
		return PatchSet{}, err
	}
	ops := diffJSONValue("", beforeValue, afterValue)
	return PatchSet{Operations: ops}, nil
}

func applyPatchOperation(document *any, operation PatchOperation) error {
	tokens, err := parseJSONPointer(operation.Path)
	if err != nil {
		return err
	}
	switch operation.Op {
	case PatchAdd:
		return patchAdd(document, tokens, operation.Value)
	case PatchReplace:
		return patchReplace(document, tokens, operation.Value)
	case PatchRemove:
		return patchRemove(document, tokens)
	default:
		return fmt.Errorf("%w: %s", ErrUnsupportedPatchOperation, operation.Op)
	}
}

func patchAdd(document *any, tokens []string, value any) error {
	updated, err := applyAt(*document, tokens, PatchAdd, value)
	if err != nil {
		return err
	}
	*document = updated
	return nil
}

func patchReplace(document *any, tokens []string, value any) error {
	updated, err := applyAt(*document, tokens, PatchReplace, value)
	if err != nil {
		return err
	}
	*document = updated
	return nil
}

func patchRemove(document *any, tokens []string) error {
	updated, err := applyAt(*document, tokens, PatchRemove, nil)
	if err != nil {
		return err
	}
	*document = updated
	return nil
}

func applyAt(current any, tokens []string, op PatchOperationType, value any) (any, error) {
	if len(tokens) == 0 {
		switch op {
		case PatchAdd, PatchReplace:
			return value, nil
		case PatchRemove:
			return nil, nil
		default:
			return nil, fmt.Errorf("%w: %s", ErrUnsupportedPatchOperation, op)
		}
	}
	key := tokens[0]
	switch target := current.(type) {
	case map[string]any:
		if len(tokens) == 1 {
			switch op {
			case PatchAdd:
				target[key] = value
			case PatchReplace:
				if _, ok := target[key]; !ok {
					return nil, fmt.Errorf("%w: %s", ErrInvalidPatchPath, pointerFromTokens(tokens))
				}
				target[key] = value
			case PatchRemove:
				if _, ok := target[key]; !ok {
					return nil, fmt.Errorf("%w: %s", ErrInvalidPatchPath, pointerFromTokens(tokens))
				}
				delete(target, key)
			default:
				return nil, fmt.Errorf("%w: %s", ErrUnsupportedPatchOperation, op)
			}
			return target, nil
		}
		child, ok := target[key]
		if !ok {
			return nil, fmt.Errorf("%w: %s", ErrInvalidPatchPath, pointerFromTokens(tokens))
		}
		updatedChild, err := applyAt(child, tokens[1:], op, value)
		if err != nil {
			return nil, err
		}
		target[key] = updatedChild
		return target, nil
	case []any:
		if len(tokens) == 1 {
			switch op {
			case PatchAdd:
				index, err := arrayIndexForAdd(key, len(target))
				if err != nil {
					return nil, err
				}
				target = append(target[:index], append([]any{value}, target[index:]...)...)
			case PatchReplace:
				index, err := arrayIndex(key, len(target))
				if err != nil {
					return nil, err
				}
				target[index] = value
			case PatchRemove:
				index, err := arrayIndex(key, len(target))
				if err != nil {
					return nil, err
				}
				target = append(target[:index], target[index+1:]...)
			default:
				return nil, fmt.Errorf("%w: %s", ErrUnsupportedPatchOperation, op)
			}
			return target, nil
		}
		index, err := arrayIndex(key, len(target))
		if err != nil {
			return nil, err
		}
		updatedChild, err := applyAt(target[index], tokens[1:], op, value)
		if err != nil {
			return nil, err
		}
		target[index] = updatedChild
		return target, nil
	default:
		return nil, fmt.Errorf("%w: %s", ErrInvalidPatchPath, pointerFromTokens(tokens))
	}
}

func parseJSONPointer(path string) ([]string, error) {
	if path == "" {
		return nil, nil
	}
	if !strings.HasPrefix(path, "/") {
		return nil, fmt.Errorf("%w: %s", ErrInvalidPatchPath, path)
	}
	rawTokens := strings.Split(path[1:], "/")
	tokens := make([]string, 0, len(rawTokens))
	for _, token := range rawTokens {
		token = strings.ReplaceAll(token, "~1", "/")
		token = strings.ReplaceAll(token, "~0", "~")
		tokens = append(tokens, token)
	}
	return tokens, nil
}

func pointerFromTokens(tokens []string) string {
	if len(tokens) == 0 {
		return ""
	}
	escaped := make([]string, 0, len(tokens))
	for _, token := range tokens {
		token = strings.ReplaceAll(token, "~", "~0")
		token = strings.ReplaceAll(token, "/", "~1")
		escaped = append(escaped, token)
	}
	return "/" + strings.Join(escaped, "/")
}

func arrayIndex(token string, length int) (int, error) {
	index, err := strconv.Atoi(token)
	if err != nil || index < 0 || index >= length {
		return 0, fmt.Errorf("%w: array index %q out of range", ErrInvalidPatchPath, token)
	}
	return index, nil
}

func arrayIndexForAdd(token string, length int) (int, error) {
	if token == "-" {
		return length, nil
	}
	index, err := strconv.Atoi(token)
	if err != nil || index < 0 || index > length {
		return 0, fmt.Errorf("%w: array add index %q out of range", ErrInvalidPatchPath, token)
	}
	return index, nil
}

func toJSONValue(value any) (any, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var out any
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func diffJSONValue(path string, before, after any) []PatchOperation {
	if reflect.DeepEqual(before, after) {
		return nil
	}
	beforeMap, beforeIsMap := before.(map[string]any)
	afterMap, afterIsMap := after.(map[string]any)
	if beforeIsMap && afterIsMap {
		ops := []PatchOperation{}
		for key := range beforeMap {
			nextPath := path + "/" + escapePointerToken(key)
			if _, ok := afterMap[key]; !ok {
				ops = append(ops, PatchOperation{Op: PatchRemove, Path: nextPath})
			}
		}
		for key, afterValue := range afterMap {
			nextPath := path + "/" + escapePointerToken(key)
			beforeValue, ok := beforeMap[key]
			if !ok {
				ops = append(ops, PatchOperation{Op: PatchAdd, Path: nextPath, Value: afterValue})
				continue
			}
			ops = append(ops, diffJSONValue(nextPath, beforeValue, afterValue)...)
		}
		return ops
	}
	if path == "" {
		return []PatchOperation{{Op: PatchReplace, Path: "", Value: after}}
	}
	return []PatchOperation{{Op: PatchReplace, Path: path, Value: after}}
}

func escapePointerToken(token string) string {
	token = strings.ReplaceAll(token, "~", "~0")
	return strings.ReplaceAll(token, "/", "~1")
}
