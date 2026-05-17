export type TransformSchemaField = {
  detail?: string;
  label: string;
  required?: boolean;
  type?: string;
  value: string;
};

export type TransformFunctionDefinition = {
  category: string;
  description: string;
  id: string;
  inputFields: TransformSchemaField[];
  name: string;
  outputFields: TransformSchemaField[];
  version: string;
};

export const transformFunctionDefinitions: TransformFunctionDefinition[] = [
  {
    id: "json.remove_fields",
    version: "1.0.0",
    name: "Remove Fields",
    category: "JSON",
    description: "Remove one or more field names from a JSON object or array. Recursive mode is useful for Feishu readonly fields such as merge_info.",
    inputFields: [
      { value: "value", label: "value", type: "object | array", required: true, detail: "Source JSON value, for example $responses.connector.feishu_convert.blocks." },
      { value: "fields", label: "fields", type: "string[]", required: true, detail: "Field names to remove, for example [\"merge_info\"]." },
      { value: "recursive", label: "recursive", type: "boolean", detail: "Remove matching fields recursively. Default: true." }
    ],
    outputFields: [
      { value: "result", label: "result", type: "object | array", detail: "Transformed JSON value." },
      { value: "removed_count", label: "removed_count", type: "integer", detail: "Number of removed fields." },
      { value: "removed_paths", label: "removed_paths", type: "string[]", detail: "Paths of removed fields." }
    ]
  },
  {
    id: "json.extract_path",
    version: "1.0.0",
    name: "Extract Path",
    category: "JSON",
    description: "Extract a nested value by dot path.",
    inputFields: [
      { value: "value", label: "value", type: "object | array", required: true, detail: "Source JSON value." },
      { value: "path", label: "path", type: "string", required: true, detail: "Dot path, for example document.document_id or 0.text." }
    ],
    outputFields: [
      { value: "result", label: "result", type: "any", detail: "Extracted value." },
      { value: "exists", label: "exists", type: "boolean", detail: "Whether the path exists." }
    ]
  },
  {
    id: "json.pick_fields",
    version: "1.0.0",
    name: "Pick Fields",
    category: "JSON",
    description: "Build a new object by copying selected field paths.",
    inputFields: [
      { value: "value", label: "value", type: "object", required: true, detail: "Source JSON object." },
      { value: "fields", label: "fields", type: "string[]", required: true, detail: "Paths to keep, for example [\"title\", \"document.id\"]." }
    ],
    outputFields: [
      { value: "result", label: "result", type: "object", detail: "Object containing selected fields." },
      { value: "picked_count", label: "picked_count", type: "integer", detail: "Number of copied top-level fields." }
    ]
  },
  {
    id: "json.rename_fields",
    version: "1.0.0",
    name: "Rename Fields",
    category: "JSON",
    description: "Rename field names in an object. Recursive mode renames matching keys in nested objects and arrays.",
    inputFields: [
      { value: "value", label: "value", type: "object | array", required: true, detail: "Source JSON value." },
      { value: "rename_map", label: "rename_map", type: "object", required: true, detail: "Map of old field name to new field name, for example {\"old\":\"new\"}." },
      { value: "recursive", label: "recursive", type: "boolean", detail: "Rename matching fields recursively. Default: false." }
    ],
    outputFields: [
      { value: "result", label: "result", type: "object | array", detail: "Renamed JSON value." },
      { value: "renamed_count", label: "renamed_count", type: "integer", detail: "Number of renamed fields." }
    ]
  },
  {
    id: "json.set_value",
    version: "1.0.0",
    name: "Set Value",
    category: "JSON",
    description: "Set a nested field to a constant or mapped value.",
    inputFields: [
      { value: "value", label: "value", type: "object", required: true, detail: "Source JSON object." },
      { value: "path", label: "path", type: "string", required: true, detail: "Target dot path." },
      { value: "new_value", label: "new_value", type: "any", required: true, detail: "Value to write." },
      { value: "create_missing", label: "create_missing", type: "boolean", detail: "Create missing parent objects. Default: true." }
    ],
    outputFields: [
      { value: "result", label: "result", type: "object", detail: "Updated object." },
      { value: "updated", label: "updated", type: "boolean", detail: "Whether the value was written." }
    ]
  },
  {
    id: "array.chunk",
    version: "1.0.0",
    name: "Chunk Array",
    category: "Array",
    description: "Split an array into batches. Useful for API limits such as Feishu's 1000-block insert limit.",
    inputFields: [
      { value: "value", label: "value", type: "array", required: true, detail: "Source array." },
      { value: "size", label: "size", type: "integer", required: true, detail: "Batch size, for example 1000." }
    ],
    outputFields: [
      { value: "chunks", label: "chunks", type: "array", detail: "Array of batches." },
      { value: "count", label: "count", type: "integer", detail: "Batch count." }
    ]
  },
  {
    id: "array.filter_empty",
    version: "1.0.0",
    name: "Filter Empty",
    category: "Array / JSON",
    description: "Remove null, empty strings, empty arrays, and empty objects.",
    inputFields: [
      { value: "value", label: "value", type: "array | object", required: true, detail: "Source value." },
      { value: "recursive", label: "recursive", type: "boolean", detail: "Filter nested values recursively. Default: true." }
    ],
    outputFields: [
      { value: "result", label: "result", type: "array | object", detail: "Filtered value." },
      { value: "removed_count", label: "removed_count", type: "integer", detail: "Number of removed empty values." }
    ]
  },
  {
    id: "string.template",
    version: "1.0.0",
    name: "String Template",
    category: "String",
    description: "Render a string with {{path}} placeholders.",
    inputFields: [
      { value: "template", label: "template", type: "string", required: true, detail: "Template string, for example Hello {{name}}." },
      { value: "variables", label: "variables", type: "object", detail: "Variables object. If empty, all transform inputs are used." }
    ],
    outputFields: [{ value: "result", label: "result", type: "string", detail: "Rendered string." }]
  },
  {
    id: "type.convert",
    version: "1.0.0",
    name: "Type Convert",
    category: "Type",
    description: "Convert a value to string, number, integer, boolean, object, or array.",
    inputFields: [
      { value: "value", label: "value", type: "any", required: true, detail: "Source value." },
      { value: "target_type", label: "target_type", type: "string", required: true, detail: "string | number | integer | boolean | object | array" }
    ],
    outputFields: [{ value: "result", label: "result", type: "any", detail: "Converted value." }]
  }
];

export function transformFunctionById(functionId: string): TransformFunctionDefinition | undefined {
  return transformFunctionDefinitions.find((fn) => fn.id === functionId);
}
