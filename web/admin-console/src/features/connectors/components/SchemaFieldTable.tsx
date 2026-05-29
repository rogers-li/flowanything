import { SchemaFieldEditor, SchemaFieldViewer } from "../../../components/schema-editor/SchemaFieldEditor";
import { createSchemaField, type SchemaFieldDraft } from "../domain";

type SchemaFieldTableProps = {
  emptyText?: string;
  fields: SchemaFieldDraft[];
  onChange: (fields: SchemaFieldDraft[]) => void;
  readOnly?: boolean;
};

export function SchemaFieldTable({ emptyText, fields, onChange, readOnly = false }: SchemaFieldTableProps) {
  return <SchemaFieldEditor createField={createSchemaField} emptyText={emptyText} fields={fields} onChange={onChange} readOnly={readOnly} />;
}

export { SchemaFieldViewer };
