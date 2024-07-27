package mapstruct

import (
	"flow-anything/utils"
	"flow-anything/valuate"
	"fmt"
	"strings"
)

type FieldType string

const (
	FieldEmpty  FieldType = ""
	FieldObject FieldType = "Object"
	FieldInt    FieldType = "Int"
	FieldFloat  FieldType = "Float"
	FieldString FieldType = "String"
	FieldArray  FieldType = "Array"
	FieldBool   FieldType = "Bool"
	FieldAny    FieldType = "Any"
)

type Field struct {
	FieldType   FieldType
	FieldName   string
	Required    bool
	Omitempty   bool
	ValueSource string
	DefaultVal  string
	SubFields   []*Field
	IsRawData   bool
	RawData     interface{}
}

func NewField(fieldName string, fieldType FieldType) *Field {
	return &Field{
		FieldType:   fieldType,
		FieldName:   fieldName,
		Required:    false,
		Omitempty:   false,
		ValueSource: "",
		DefaultVal:  "",
		SubFields:   nil,
		IsRawData:   false,
		RawData:     nil,
	}
}

// ParseFieldEasyConfig 简化版的配置格式为 "type:String,required,omitempty,value:expression,default:defaultExpression"
// 每个配置属性以英文逗号分隔，配置属性与顺序无关
func ParseFieldEasyConfig(fieldName string, fieldConfig interface{}) (*Field, error) {
	field := NewField(fieldName, "")
	fieldConfigStr, ok := fieldConfig.(string)
	// 如果字段配置不是string，或者不包含“type:”,则认为这个配置是字段值，不需要解析字段定义
	// 如果后续字段值为包含“type:"的字符串，则这里的逻辑要改
	if !ok || !strings.Contains(fieldConfigStr, "type:") {
		field.RawData = fieldConfig
		field.IsRawData = true
		return field, nil
	}
	strArray := strings.Split(fieldConfigStr, ",")
	for _, str := range strArray {
		if "required" == strings.ToLower(str) {
			field.Required = true
		} else if "omitempty" == strings.ToLower(str) {
			field.Omitempty = true
		} else if strings.HasPrefix(str, "type:") {
			fieldType := strings.ReplaceAll(str, "type:", "")
			field.FieldType = FieldType(fieldType)
		} else if strings.HasPrefix(str, "value:") {
			expression := strings.ReplaceAll(str, "value:", "")
			field.ValueSource = expression
		} else if strings.HasPrefix(str, "default:") {
			expression := strings.ReplaceAll(str, "default:", "")
			field.DefaultVal = expression
		}
	}
	return field, nil
}

func (f *Field) GetValue(sourceMap map[string]interface{}) (result interface{}, err error) {
	if f.IsRawData {
		return f.RawData, nil
	}
	return f.getValueByExpression(sourceMap)
}

// 使用表达式引擎获取字段值
func (f *Field) getValueByExpression(sourceMap map[string]interface{}) (result interface{}, err error) {
	expr, err := valuate.Expression(f.ValueSource)
	if err == nil {
		result, err = expr.Evaluate(sourceMap)
		// 如果字段为必填，通过value source表达式获取不到值，则尝试使用默认值表达式获取值
		if err != nil && f.Required && f.DefaultVal != "" {
			expr, err = valuate.Expression(f.DefaultVal)
			if err != nil {
				return nil, err
			}
			result, err = expr.Evaluate(sourceMap)
			if err != nil {
				return nil, err
			}
		}
	}
	// 如果字段为非必填，则直接返回结果
	if !f.Required {
		return f.anyValue(result), nil
	}
	// 如果字段为必填，并且结果值为nil，则报错找不到字段值
	if result == nil {
		return nil, fmt.Errorf("field value not found,value:" + f.ValueSource + " default:" + f.DefaultVal)
	}
	// 如果字段不允许为空，并且值结果为空，则报错字段值不能为空
	if !f.Omitempty && utils.IsEmptyVal(result) {
		return nil, fmt.Errorf("field value can not be empty:" + f.ValueSource + " default:" + f.DefaultVal)
	}
	return f.anyValue(result), nil
}

// ConvertType todo 需要增加字段类型转换
func (f *Field) ConvertType(value interface{}) (interface{}, error) {
	// 需要把表达式引擎的返回值提取出来，否则json序列化会丢失字段
	v, ok := value.([]valuate.Value)
	if ok {
		resultArray := make([]interface{}, 0)
		for _, item := range v {
			resultArray = append(resultArray, item.Get())
		}
		return resultArray, nil
	}
	return value, nil
}

func (f *Field) anyValue(value interface{}) interface{} {
	if value == nil {
		return f.initValue()
	} else if array, ok := value.([]interface{}); ok && len(array) == 0 { // 由于可能会返回[]interface(nil),需要重新初始化空数组
		return f.initValue()
	}
	if v, ok := value.(valuate.Array); ok {
		valuate.ArrayValue(v)
	}
	if v, ok := value.(valuate.Value); ok {
		return v.Get()
	}
	return value
}

func (f *Field) initValue() interface{} {
	switch f.FieldType {
	case FieldObject:
		return map[string]interface{}{}
	case FieldInt:
		return 0
	case FieldFloat:
		return 0
	case FieldString:
		return ""
	case FieldArray:
		return []interface{}{}
	case FieldBool:
		return false
	default:
		return nil
	}
}
