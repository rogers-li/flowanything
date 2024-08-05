package variable

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
