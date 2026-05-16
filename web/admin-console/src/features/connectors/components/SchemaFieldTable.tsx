import { Fragment } from "react";
import { createSchemaField, type SchemaFieldDraft, type SchemaFieldType } from "../domain";

type SchemaFieldTableProps = {
  fields: SchemaFieldDraft[];
  onChange: (fields: SchemaFieldDraft[]) => void;
  readOnly?: boolean;
};

const fieldTypes: SchemaFieldType[] = ["string", "number", "integer", "boolean", "object", "array"];

export function SchemaFieldTable({ fields, onChange, readOnly = false }: SchemaFieldTableProps) {
  const updateField = (fieldID: string, patch: Partial<SchemaFieldDraft>) => {
    if (readOnly) return;
    onChange(fields.map((field) => (field.id === fieldID ? { ...field, ...patch } : field)));
  };

  const updateFieldType = (fieldID: string, type: SchemaFieldType) => {
    if (readOnly) return;
    onChange(
      fields.map((field) => {
        if (field.id !== fieldID) return field;
        if (type === "object") {
          return { ...field, type, children: field.children ?? [] };
        }
        if (type === "array") {
          return { ...field, type, arrayItemType: field.arrayItemType ?? "string", children: field.children ?? [] };
        }
        return { ...field, type, arrayItemType: undefined, children: undefined };
      })
    );
  };

  const updateArrayItemType = (fieldID: string, arrayItemType: SchemaFieldType) => {
    if (readOnly) return;
    onChange(
      fields.map((field) =>
        field.id === fieldID
          ? {
              ...field,
              arrayItemType,
              children: arrayItemType === "object" ? field.children ?? [] : undefined
            }
          : field
      )
    );
  };

  const updateChildren = (fieldID: string, children: SchemaFieldDraft[]) => {
    if (readOnly) return;
    updateField(fieldID, { children });
  };

  const removeField = (fieldID: string) => {
    if (readOnly) return;
    onChange(fields.filter((field) => field.id !== fieldID));
  };

  return (
    <div className="schema-table">
      <table>
        <thead>
          <tr>
            <th>Field</th>
            <th>Type</th>
            <th>Required</th>
            <th>Description</th>
            <th>Action</th>
          </tr>
        </thead>
        <tbody>
          {fields.map((field) => (
            <Fragment key={field.id}>
              <tr>
                <td>
                  <input disabled={readOnly} value={field.name} onChange={(event) => updateField(field.id, { name: event.target.value })} />
                </td>
                <td>
                  <div className="schema-type-stack">
                    <select
                      value={field.type}
                      disabled={readOnly}
                      onChange={(event) => updateFieldType(field.id, event.target.value as SchemaFieldType)}
                    >
                      {fieldTypes.map((fieldType) => (
                        <option key={fieldType} value={fieldType}>
                          {fieldType}
                        </option>
                      ))}
                    </select>
                    {field.type === "array" ? (
                      <select
                        aria-label={`${field.name || "array"} item type`}
                        value={field.arrayItemType ?? "string"}
                        disabled={readOnly}
                        onChange={(event) => updateArrayItemType(field.id, event.target.value as SchemaFieldType)}
                      >
                        {fieldTypes.map((fieldType) => (
                          <option key={fieldType} value={fieldType}>
                            {`items: ${fieldType}`}
                          </option>
                        ))}
                      </select>
                    ) : null}
                  </div>
                </td>
                <td>
                  <label className="checkbox-field">
                    <input
                      checked={field.required}
                      disabled={readOnly}
                      type="checkbox"
                      onChange={(event) => updateField(field.id, { required: event.target.checked })}
                    />
                    Required
                  </label>
                </td>
                <td>
                  <input
                    value={field.description}
                    disabled={readOnly}
                    onChange={(event) => updateField(field.id, { description: event.target.value })}
                  />
                </td>
                <td>
                  <button className="secondary-action" type="button" onClick={() => removeField(field.id)} disabled={readOnly}>
                    Remove
                  </button>
                </td>
              </tr>
              {fieldSupportsChildren(field) ? (
                <tr className="schema-nested-row" key={`${field.id}-nested`}>
                  <td colSpan={5}>
                    <div className="schema-nested-panel">
                      <span>{field.type === "array" ? `${field.name || "array"}[] fields` : `${field.name || "object"} fields`}</span>
                      <SchemaFieldTable fields={field.children ?? []} onChange={(children) => updateChildren(field.id, children)} readOnly={readOnly} />
                    </div>
                  </td>
                </tr>
              ) : null}
            </Fragment>
          ))}
        </tbody>
      </table>
      {readOnly ? null : (
        <button className="secondary-action" type="button" onClick={() => onChange([...fields, createSchemaField()])}>
          Add Field
        </button>
      )}
    </div>
  );
}

function fieldSupportsChildren(field: SchemaFieldDraft): boolean {
  return field.type === "object" || (field.type === "array" && (field.arrayItemType ?? "string") === "object");
}
