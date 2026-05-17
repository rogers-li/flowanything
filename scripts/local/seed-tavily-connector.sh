#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
ENV_FILE="${FLOW_ANYTHING_ENV_FILE:-$ROOT_DIR/configs/local/services.env}"
PLATFORM_API_URL="${PLATFORM_API_URL:-http://localhost:8080}"
TENANT_ID="${FLOW_ANYTHING_TENANT_ID:-tenant_1}"

load_env_file() {
  if [[ ! -f "$ENV_FILE" ]]; then
    return
  fi

  while IFS= read -r line || [[ -n "$line" ]]; do
    line="${line#"${line%%[![:space:]]*}"}"
    line="${line%"${line##*[![:space:]]}"}"
    [[ -z "$line" || "$line" == \#* ]] && continue
    [[ "$line" != *=* ]] && continue
    export "$line"
  done < "$ENV_FILE"
}

json_request() {
  local method="$1"
  local path="$2"
  local payload="$3"

  curl -fsS -X "$method" "$PLATFORM_API_URL$path" \
    -H 'Content-Type: application/json' \
    -d "$payload" >/dev/null
}

resource_exists() {
  local path="$1"
  curl -fsS "$PLATFORM_API_URL$path" >/dev/null 2>&1
}

upsert_json() {
  local label="$1"
  local get_path="$2"
  local post_path="$3"
  local put_path="$4"
  local payload="$5"

  if resource_exists "$get_path"; then
    echo "update $label"
    json_request PUT "$put_path" "$payload"
    return
  fi

  echo "create $label"
  json_request POST "$post_path" "$payload"
}

main() {
  load_env_file

  local connector_payload
  connector_payload=$(cat <<'JSON'
{
  "id": "conn_tavily",
  "tenant_id": "tenant_1",
  "name": "Tavily Web Intelligence",
  "description": "Tavily web search, extraction, crawl, and site map API connector.",
  "business_domain": "Search",
  "owner_team": "AI Platform",
  "type": "http",
  "status": "enabled",
  "base_url": "https://api.tavily.com",
  "headers": {
    "Accept": "application/json"
  },
  "auth": {
    "type": "bearer",
    "secret_ref": "env:TAVILY_API_KEY"
  },
  "timeout_ms": 15000,
  "version": "v1"
}
JSON
)
  connector_payload="${connector_payload//\"tenant_1\"/\"$TENANT_ID\"}"

  local operation_payload
  operation_payload=$(cat <<'JSON'
{
  "id": "connop_tavily_search",
  "tenant_id": "tenant_1",
  "connector_id": "conn_tavily",
  "name": "tavily_search",
  "description": "Search the web with Tavily and return ranked results.",
  "type": "http",
  "status": "enabled",
  "implementation_mode": "simple_http",
  "method": "POST",
  "path": "/search",
  "input_schema": {
    "type": "object",
    "required": ["query"],
    "properties": {
      "query": {
        "type": "string",
        "description": "The search query to execute."
      },
      "search_depth": {
        "type": "string",
        "enum": ["basic", "advanced", "fast", "ultra-fast"],
        "description": "Latency vs relevance tradeoff. Defaults to basic."
      },
      "topic": {
        "type": "string",
        "enum": ["general", "news", "finance"],
        "description": "Search category. Defaults to general."
      },
      "chunks_per_source": {
        "type": "integer",
        "description": "Maximum relevant chunks per source when using advanced search."
      },
      "max_results": {
        "type": "integer",
        "description": "Maximum number of search results to return."
      },
      "time_range": {
        "type": "string",
        "enum": ["day", "week", "month", "year", "d", "w", "m", "y"],
        "description": "Filter results by publish or update time range."
      },
      "start_date": {
        "type": "string",
        "description": "Return results after this date, formatted as YYYY-MM-DD."
      },
      "end_date": {
        "type": "string",
        "description": "Return results before this date, formatted as YYYY-MM-DD."
      },
      "include_answer": {
        "description": "Whether to include a generated answer. Tavily accepts false, true, basic, or advanced."
      },
      "include_raw_content": {
        "description": "Whether to include parsed page content. Tavily accepts false, true, markdown, or text."
      },
      "include_images": {
        "type": "boolean",
        "description": "Whether to include image results."
      },
      "include_image_descriptions": {
        "type": "boolean",
        "description": "Whether image results should include descriptions."
      },
      "include_favicon": {
        "type": "boolean",
        "description": "Whether to include favicon URLs for each result."
      },
      "include_domains": {
        "type": "array",
        "items": { "type": "string" },
        "description": "Domains that should be included in the search results."
      },
      "exclude_domains": {
        "type": "array",
        "items": { "type": "string" },
        "description": "Domains that should be excluded from the search results."
      },
      "country": {
        "type": "string",
        "description": "Country boost for general search, for example china, singapore, or united states."
      },
      "auto_parameters": {
        "type": "boolean",
        "description": "Let Tavily infer topic/search_depth from the query when appropriate."
      },
      "include_usage": {
        "type": "boolean",
        "description": "Whether to include API credit usage."
      }
    }
  },
  "output_schema": {
    "type": "object",
    "properties": {
      "query": { "type": "string" },
      "answer": { "type": "string" },
      "results": {
        "type": "array",
        "items": {
          "type": "object",
          "properties": {
            "title": { "type": "string" },
            "url": { "type": "string" },
            "content": { "type": "string" },
            "score": { "type": "number" },
            "raw_content": {},
            "favicon": { "type": "string" }
          }
        }
      },
      "images": { "type": "array" },
      "response_time": {},
      "auto_parameters": { "type": "object" },
      "usage": { "type": "object" },
      "request_id": { "type": "string" },
      "status_code": { "type": "integer" }
    }
  },
  "timeout_ms": 15000,
  "version": "v1"
}
JSON
)
  operation_payload="${operation_payload//\"tenant_1\"/\"$TENANT_ID\"}"

  local extract_operation_payload
  extract_operation_payload=$(cat <<'JSON'
{
  "id": "connop_tavily_extract",
  "tenant_id": "tenant_1",
  "connector_id": "conn_tavily",
  "name": "tavily_extract",
  "description": "Extract clean markdown or text content from one or more web pages.",
  "type": "http",
  "status": "enabled",
  "implementation_mode": "simple_http",
  "method": "POST",
  "path": "/extract",
  "input_schema": {
    "type": "object",
    "required": ["urls"],
    "properties": {
      "urls": {
        "type": "array",
        "items": { "type": "string" },
        "description": "One or more URLs to extract content from."
      },
      "query": {
        "type": "string",
        "description": "Optional user intent for reranking extracted content chunks."
      },
      "chunks_per_source": {
        "type": "integer",
        "description": "Maximum relevant chunks per source when query is provided. Valid range: 1 to 5."
      },
      "extract_depth": {
        "type": "string",
        "enum": ["basic", "advanced"],
        "description": "Extraction depth. basic is cheaper; advanced retrieves more data such as tables and embedded content."
      },
      "include_images": {
        "type": "boolean",
        "description": "Whether to include extracted image URLs."
      },
      "include_favicon": {
        "type": "boolean",
        "description": "Whether to include the favicon URL for each result."
      },
      "format": {
        "type": "string",
        "enum": ["markdown", "text"],
        "description": "Content format returned by Tavily. Defaults to markdown."
      },
      "timeout": {
        "type": "number",
        "description": "Maximum extraction time in seconds. Valid range: 1 to 60."
      },
      "include_usage": {
        "type": "boolean",
        "description": "Whether to include API credit usage."
      }
    }
  },
  "output_schema": {
    "type": "object",
    "properties": {
      "results": {
        "type": "array",
        "description": "Extracted content from the provided URLs.",
        "items": {
          "type": "object",
          "properties": {
            "url": { "type": "string", "description": "Source URL." },
            "raw_content": { "type": "string", "description": "Extracted page content in markdown or text." },
            "images": {
              "type": "array",
              "items": { "type": "string" },
              "description": "Extracted image URLs when include_images is enabled."
            },
            "favicon": { "type": "string", "description": "Favicon URL when include_favicon is enabled." }
          }
        }
      },
      "failed_results": {
        "type": "array",
        "description": "URLs that could not be processed.",
        "items": {
          "type": "object",
          "properties": {
            "url": { "type": "string" },
            "error": { "type": "string" }
          }
        }
      },
      "response_time": { "type": "number", "description": "Request duration in seconds." },
      "usage": {
        "type": "object",
        "properties": {
          "credits": { "type": "number", "description": "API credits used by this request." }
        }
      },
      "request_id": { "type": "string", "description": "Tavily request identifier." },
      "status_code": { "type": "integer", "description": "HTTP status code captured by connector runtime." }
    }
  },
  "timeout_ms": 60000,
  "version": "v1"
}
JSON
)
  extract_operation_payload="${extract_operation_payload//\"tenant_1\"/\"$TENANT_ID\"}"

  local crawl_operation_payload
  crawl_operation_payload=$(cat <<'JSON'
{
  "id": "connop_tavily_crawl",
  "tenant_id": "tenant_1",
  "connector_id": "conn_tavily",
  "name": "tavily_crawl",
  "description": "Crawl a website from a root URL and extract content from discovered pages.",
  "type": "http",
  "status": "enabled",
  "implementation_mode": "simple_http",
  "method": "POST",
  "path": "/crawl",
  "input_schema": {
    "type": "object",
    "required": ["url"],
    "properties": {
      "url": {
        "type": "string",
        "description": "The root URL to begin the crawl."
      },
      "instructions": {
        "type": "string",
        "description": "Natural language instructions for the crawler, for example 'Find all pages about the Python SDK'."
      },
      "max_depth": {
        "type": "integer",
        "description": "Maximum crawl depth from the base URL. Valid range: 1 to 5."
      },
      "max_breadth": {
        "type": "integer",
        "description": "Maximum number of links to follow per level. Valid range: 1 to 500."
      },
      "limit": {
        "type": "integer",
        "description": "Total number of links the crawler will process before stopping."
      },
      "select_paths": {
        "type": "array",
        "items": { "type": "string" },
        "description": "Regex path patterns to include, for example /docs/.*."
      },
      "select_domains": {
        "type": "array",
        "items": { "type": "string" },
        "description": "Regex domain patterns to include, for example ^docs\\.example\\.com$."
      },
      "exclude_paths": {
        "type": "array",
        "items": { "type": "string" },
        "description": "Regex path patterns to exclude, for example /private/.*."
      },
      "exclude_domains": {
        "type": "array",
        "items": { "type": "string" },
        "description": "Regex domain patterns to exclude."
      },
      "allow_external": {
        "type": "boolean",
        "description": "Whether to include external domain links in final results."
      },
      "include_images": {
        "type": "boolean",
        "description": "Whether to include images in crawl results."
      },
      "extract_depth": {
        "type": "string",
        "enum": ["basic", "advanced"],
        "description": "Extraction depth used for crawled pages."
      },
      "format": {
        "type": "string",
        "enum": ["markdown", "text"],
        "description": "Extracted page content format."
      },
      "include_favicon": {
        "type": "boolean",
        "description": "Whether to include the favicon URL for each result."
      },
      "timeout": {
        "type": "number",
        "description": "Maximum crawl time in seconds. Valid range: 10 to 150."
      },
      "include_usage": {
        "type": "boolean",
        "description": "Whether to include API credit usage."
      }
    }
  },
  "output_schema": {
    "type": "object",
    "properties": {
      "base_url": { "type": "string", "description": "The base URL that was crawled." },
      "results": {
        "type": "array",
        "description": "Extracted content from crawled URLs.",
        "items": {
          "type": "object",
          "properties": {
            "url": { "type": "string", "description": "Crawled page URL." },
            "raw_content": { "type": "string", "description": "Extracted content from the page." },
            "images": {
              "type": "array",
              "items": { "type": "string" },
              "description": "Image URLs when include_images is enabled."
            },
            "favicon": { "type": "string", "description": "Favicon URL when include_favicon is enabled." }
          }
        }
      },
      "response_time": { "type": "number", "description": "Request duration in seconds." },
      "usage": {
        "type": "object",
        "properties": {
          "credits": { "type": "number", "description": "API credits used by this request." }
        }
      },
      "request_id": { "type": "string", "description": "Tavily request identifier." },
      "status_code": { "type": "integer", "description": "HTTP status code captured by connector runtime." }
    }
  },
  "timeout_ms": 150000,
  "version": "v1"
}
JSON
)
  crawl_operation_payload="${crawl_operation_payload//\"tenant_1\"/\"$TENANT_ID\"}"

  local map_operation_payload
  map_operation_payload=$(cat <<'JSON'
{
  "id": "connop_tavily_map",
  "tenant_id": "tenant_1",
  "connector_id": "conn_tavily",
  "name": "tavily_map",
  "description": "Map a website from a root URL and return discovered page URLs.",
  "type": "http",
  "status": "enabled",
  "implementation_mode": "simple_http",
  "method": "POST",
  "path": "/map",
  "input_schema": {
    "type": "object",
    "required": ["url"],
    "properties": {
      "url": {
        "type": "string",
        "description": "The root URL to begin the mapping."
      },
      "instructions": {
        "type": "string",
        "description": "Natural language instructions for site discovery, for example 'Find all pages about the Python SDK'."
      },
      "max_depth": {
        "type": "integer",
        "description": "Maximum mapping depth from the base URL. Valid range: 1 to 5."
      },
      "max_breadth": {
        "type": "integer",
        "description": "Maximum number of links to follow per level. Valid range: 1 to 500."
      },
      "limit": {
        "type": "integer",
        "description": "Total number of links the mapper will process before stopping."
      },
      "select_paths": {
        "type": "array",
        "items": { "type": "string" },
        "description": "Regex path patterns to include, for example /docs/.*."
      },
      "select_domains": {
        "type": "array",
        "items": { "type": "string" },
        "description": "Regex domain patterns to include, for example ^docs\\.example\\.com$."
      },
      "exclude_paths": {
        "type": "array",
        "items": { "type": "string" },
        "description": "Regex path patterns to exclude, for example /private/.*."
      },
      "exclude_domains": {
        "type": "array",
        "items": { "type": "string" },
        "description": "Regex domain patterns to exclude."
      },
      "allow_external": {
        "type": "boolean",
        "description": "Whether to include external domain links in final results."
      },
      "timeout": {
        "type": "number",
        "description": "Maximum mapping time in seconds. Valid range: 10 to 150."
      },
      "include_usage": {
        "type": "boolean",
        "description": "Whether to include API credit usage."
      }
    }
  },
  "output_schema": {
    "type": "object",
    "properties": {
      "base_url": { "type": "string", "description": "The base URL that was mapped." },
      "results": {
        "type": "array",
        "items": { "type": "string" },
        "description": "URLs discovered during mapping."
      },
      "response_time": { "type": "number", "description": "Request duration in seconds." },
      "usage": {
        "type": "object",
        "properties": {
          "credits": { "type": "number", "description": "API credits used by this request." }
        }
      },
      "request_id": { "type": "string", "description": "Tavily request identifier." },
      "status_code": { "type": "integer", "description": "HTTP status code captured by connector runtime." }
    }
  },
  "timeout_ms": 150000,
  "version": "v1"
}
JSON
)
  map_operation_payload="${map_operation_payload//\"tenant_1\"/\"$TENANT_ID\"}"

  upsert_json \
    "Tavily connector" \
    "/v1/connectors/conn_tavily?tenant_id=$TENANT_ID" \
    "/v1/connectors" \
    "/v1/connectors/conn_tavily?tenant_id=$TENANT_ID" \
    "$connector_payload"

  upsert_json \
    "Tavily search operation" \
    "/v1/connector-operations/connop_tavily_search?tenant_id=$TENANT_ID" \
    "/v1/connector-operations" \
    "/v1/connector-operations/connop_tavily_search?tenant_id=$TENANT_ID" \
    "$operation_payload"

  upsert_json \
    "Tavily extract operation" \
    "/v1/connector-operations/connop_tavily_extract?tenant_id=$TENANT_ID" \
    "/v1/connector-operations" \
    "/v1/connector-operations/connop_tavily_extract?tenant_id=$TENANT_ID" \
    "$extract_operation_payload"

  upsert_json \
    "Tavily crawl operation" \
    "/v1/connector-operations/connop_tavily_crawl?tenant_id=$TENANT_ID" \
    "/v1/connector-operations" \
    "/v1/connector-operations/connop_tavily_crawl?tenant_id=$TENANT_ID" \
    "$crawl_operation_payload"

  upsert_json \
    "Tavily map operation" \
    "/v1/connector-operations/connop_tavily_map?tenant_id=$TENANT_ID" \
    "/v1/connector-operations" \
    "/v1/connector-operations/connop_tavily_map?tenant_id=$TENANT_ID" \
    "$map_operation_payload"

  echo "tavily connector seeded."
  echo "secret_ref: env:TAVILY_API_KEY"
  echo "operations: connop_tavily_search connop_tavily_extract connop_tavily_crawl connop_tavily_map"
}

main "$@"
