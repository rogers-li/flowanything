package model

type FieldType string

const (
	FieldEmpty    FieldType = ""
	FieldObject   FieldType = "Object"
	FieldInt      FieldType = "Int"
	FieldFloat    FieldType = "Float"
	FieldString   FieldType = "String"
	FieldArray    FieldType = "Array"
	FieldBool     FieldType = "Bool"
	FieldPathList FieldType = "PathList"
	FieldOperator FieldType = "Operator"
	FieldAny      FieldType = "Any"
)

var AllType = map[FieldType]bool{
	FieldObject:   true,
	FieldInt:      true,
	FieldString:   true,
	FieldArray:    true,
	FieldBool:     true,
	FieldPathList: true,
	FieldOperator: true,
	FieldAny:      true,
}

type Field struct {
	FieldName      string      `json:"field_name"`
	FieldType      FieldType   `json:"field_type"`
	Required       bool        `json:"required"`
	ForbiddenEmpty bool        `json:"forbidden_empty"`
	SubFields      []*Field    `json:"sub_fields"`
	DefaultVal     interface{} `json:"default_val"`
}

type FieldWithInitExpression struct {
	Field
	FieldExpression      string `json:"field_expression"`
	DefaultValExpression string `json:"default_val_expression"`
}
