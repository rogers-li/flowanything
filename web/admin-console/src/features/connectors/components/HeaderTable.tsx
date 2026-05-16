import { createHeader, type HeaderDraft } from "../domain";

type HeaderTableProps = {
  headers: HeaderDraft[];
  onChange: (headers: HeaderDraft[]) => void;
};

export function HeaderTable({ headers, onChange }: HeaderTableProps) {
  const updateHeader = (headerID: string, patch: Partial<HeaderDraft>) => {
    onChange(headers.map((header) => (header.id === headerID ? { ...header, ...patch } : header)));
  };

  const removeHeader = (headerID: string) => {
    onChange(headers.filter((header) => header.id !== headerID));
  };

  return (
    <div className="schema-table">
      <table>
        <thead>
          <tr>
            <th>Header</th>
            <th>Value</th>
            <th>Action</th>
          </tr>
        </thead>
        <tbody>
          {headers.map((header) => (
            <tr key={header.id}>
              <td>
                <input value={header.name} onChange={(event) => updateHeader(header.id, { name: event.target.value })} />
              </td>
              <td>
                <input value={header.value} onChange={(event) => updateHeader(header.id, { value: event.target.value })} />
              </td>
              <td>
                <button className="secondary-action" type="button" onClick={() => removeHeader(header.id)}>
                  Remove
                </button>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
      <button className="secondary-action" type="button" onClick={() => onChange([...headers, createHeader()])}>
        Add Header
      </button>
    </div>
  );
}
