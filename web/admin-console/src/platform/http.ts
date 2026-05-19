export type JsonMethod = "POST" | "PUT" | "PATCH" | "DELETE";

export class ApiError extends Error {
  constructor(
    message: string,
    readonly status: number,
    readonly statusText: string,
    readonly url: string,
    readonly detail?: unknown
  ) {
    super(message);
    this.name = "ApiError";
  }
}

export async function getJson<T>(url: string): Promise<T> {
  return requestJson<T>(url, {
    method: "GET",
    headers: {
      Accept: "application/json"
    }
  });
}

export async function sendJson<T>(url: string, method: JsonMethod, body?: unknown): Promise<T> {
  return requestJson<T>(url, {
    method,
    headers: {
      Accept: "application/json",
      "Content-Type": "application/json"
    },
    body: body === undefined ? undefined : JSON.stringify(body)
  });
}

async function requestJson<T>(url: string, init: RequestInit): Promise<T> {
  const response = await fetch(url, init);
  if (!response.ok) {
    throw await apiError(response, url);
  }
  if (response.status === 204) {
    return undefined as T;
  }
  return response.json() as Promise<T>;
}

async function apiError(response: Response, url: string): Promise<ApiError> {
  const detail = await readErrorDetail(response);
  const detailMessage = errorMessageFromDetail(detail);
  const suffix = detailMessage ? `: ${detailMessage}` : "";
  return new ApiError(`Request failed: ${response.status} ${response.statusText} (${url})${suffix}`, response.status, response.statusText, url, detail);
}

async function readErrorDetail(response: Response): Promise<unknown> {
  const text = await response.text();
  if (!text) return undefined;
  try {
    return JSON.parse(text) as unknown;
  } catch {
    return text.slice(0, 300);
  }
}

function errorMessageFromDetail(detail: unknown): string {
  if (!detail) return "";
  if (typeof detail === "string") return detail;
  if (typeof detail !== "object") return String(detail);
  const body = detail as { error?: { code?: string; message?: string }; message?: string };
  if (body.error?.message) {
    return body.error.code ? `${body.error.code} - ${body.error.message}` : body.error.message;
  }
  if (body.message) return body.message;
  return "";
}
