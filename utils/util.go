package utils

import (
	"encoding/json"
	"errors"
	"math/big"
	"reflect"
	"strconv"
	"strings"
)

func ReConstruct(a interface{}, b interface{}) error {
	buf, err := json.Marshal(a)
	if err != nil {
		return err
	}
	err = json.Unmarshal(buf, &b)
	if err != nil {
		return err
	}
	return nil
}
func IsKind(what interface{}, kinds ...reflect.Kind) bool {
	target := what
	if isJSONNumber(what) {
		// JSON Numbers are strings!
		target = *mustBeNumber(what)
	}
	targetKind := reflect.ValueOf(target).Kind()
	for _, kind := range kinds {
		if targetKind == kind {
			return true
		}
	}
	return false
}

func isJSONNumber(what interface{}) bool {

	switch what.(type) {

	case json.Number:
		return true
	}

	return false
}

func mustBeNumber(what interface{}) *big.Rat {

	if isJSONNumber(what) {
		number := what.(json.Number)
		float64Value, success := new(big.Rat).SetString(string(number))
		if success {
			return float64Value
		}
	}

	return nil

}

func GetValByPath(content interface{}, path string) (interface{}, error) {
	strs := strings.Split(path, ".")
	result := content
	for _, str := range strs {
		if !IsKind(result, reflect.Map) {
			return nil, errors.New("not a map")
		}
		curMap := result.(map[string]interface{})
		cur, ok := curMap[str]
		if !ok {
			return nil, errors.New("has no key ：" + str)
		}
		result = cur
	}
	return result, nil
}

func ToString(in interface{}) string {
	if numberVal, ok := in.(json.Number); ok {
		return numberVal.String()
	}
	if numberVal, ok := in.(int64); ok {
		return strconv.FormatInt(numberVal, 10)
	}
	if strVal, ok := in.(string); ok {
		return strVal
	}
	return ""
}
