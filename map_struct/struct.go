package mapstruct

import (
	"encoding/json"
	"flow-anything/utils"
	"flow-anything/variable"
	"reflect"
)

// Build 根据映射配置的结构体，从数据原map中提取值并组装结果结构体
func Build(field *variable.Field, sourceMap map[string]interface{}) (interface{}, error) {
	if field == nil {
		return nil, nil
	}
	if len(field.SubFields) > 0 {
		result := make(map[string]interface{})
		for _, sub := range field.SubFields {
			subValue, err := Build(sub, sourceMap)
			if err != nil {
				return nil, err
			}
			result[sub.FieldName] = subValue
		}
		return result, nil
	}
	return GetValue(field, sourceMap)
}

// ParseStructConfig 解析配置的字段映射json字符串
func ParseStructConfig(structConfig string) (*variable.Field, error) {
	var config map[string]interface{}
	err := json.Unmarshal([]byte(structConfig), &config)
	if err != nil {
		return nil, err
	}
	return parseStruct("ROOT", config)
}

func parseStruct(fieldName string, fieldConfig interface{}) (*variable.Field, error) {
	if utils.IsKind(fieldConfig, reflect.Map) {
		objectMap, _ := fieldConfig.(map[string]interface{})
		// 如果配置的是map，则解析为object
		resultField := variable.NewField(fieldName, variable.FieldObject)
		subFields := make([]*variable.Field, 0)
		// 遍历map的元素，解析为sub fields
		for subK, subV := range objectMap {
			subField, err := parseStruct(subK, subV)
			if err != nil {
				return nil, err
			}
			subFields = append(subFields, subField)
		}
		resultField.SubFields = subFields
		return resultField, nil
	} else {
		return ParseFieldEasyConfig(fieldName, fieldConfig)
	}
}
