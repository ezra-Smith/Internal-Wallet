#!/usr/bin/env bash

# Sync grpc-gateway OpenAPI HTTP mappings from api-gateway/routes.yaml.
#
# Generates docs/swagger/*_http.yaml for every service defined in routes.yaml:
#   - selector: <protoPackage>.<protoService>.<rpcMethod>
#   - <httpVerb>: <path>
#   - body: "*" (for non-GET/DELETE)
#
# Also generates docs/swagger/*.swagger.json via protoc-gen-openapiv2, and normalizes:
#   .schemes = ["http","https"]
#
# Usage:
#   ./scripts/sync-swagger-http-yaml.sh
#   ./scripts/sync-swagger-http-yaml.sh path/to/routes.yaml path/to/output_dir

set -euo pipefail

ROUTES_FILE="${1:-api-gateway/routes.yaml}"
OUT_DIR="${2:-docs/swagger}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$PROJECT_ROOT"

if ! command -v yq >/dev/null 2>&1; then
  echo "[error] yq is required (https://github.com/mikefarah/yq/)" >&2
  exit 1
fi

if ! command -v go >/dev/null 2>&1; then
  echo "[error] go is required" >&2
  exit 1
fi

if ! command -v protoc >/dev/null 2>&1; then
  echo "[error] protoc is required" >&2
  exit 1
fi

GO_BIN_DIR="$(go env GOBIN 2>/dev/null || true)"
if [[ -z "$GO_BIN_DIR" ]]; then
  GO_BIN_DIR="$(go env GOPATH 2>/dev/null || true)/bin"
fi
export PATH="$GO_BIN_DIR:$PATH"

if ! command -v protoc-gen-openapiv2 >/dev/null 2>&1; then
  echo "[warn] protoc-gen-openapiv2 not found; installing grpc-gateway OpenAPI plugin..." >&2
  if ! go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2@latest; then
    echo "[error] failed to install protoc-gen-openapiv2 via 'go install'" >&2
    exit 1
  fi
fi

if ! command -v protoc-gen-openapiv2 >/dev/null 2>&1; then
  echo "[error] protoc-gen-openapiv2 is required but still not found in PATH (expected under: $GO_BIN_DIR)" >&2
  exit 1
fi

if ! command -v jq >/dev/null 2>&1; then
  echo "[error] jq is required" >&2
  exit 1
fi

if [[ ! -f "$ROUTES_FILE" ]]; then
  echo "[error] routes file not found: $ROUTES_FILE" >&2
  exit 1
fi

# Keep the Kubernetes kustomize copy in sync.
# Why: kubectl/kustomize has a load restrictor and cannot read files outside the kustomization root,
# so production manifests use a checked-in copy under deploy/k8s/.
if [[ "$ROUTES_FILE" == "api-gateway/routes.yaml" ]]; then
  K8S_ROUTES_COPY="$PROJECT_ROOT/deploy/k8s/base/config/files/api-gateway/routes.yaml"
  mkdir -p "$(dirname "$K8S_ROUTES_COPY")"
  cp "$ROUTES_FILE" "$K8S_ROUTES_COPY"
  echo "[info] synced k8s routes copy: $K8S_ROUTES_COPY" >&2
fi

mkdir -p "$OUT_DIR"

GW_DIR="$(go list -m -f '{{.Dir}}' github.com/grpc-ecosystem/grpc-gateway/v2 2>/dev/null || true)"
if [[ -z "$GW_DIR" ]]; then
  echo "[error] failed to locate grpc-gateway module dir via go list; try running 'go mod download' first" >&2
  exit 1
fi

TMP_FILES=()
cleanup() {
  for f in "${TMP_FILES[@]}"; do
    if [[ -f "$f" ]]; then
      rm -f "$f"
    fi
  done
}
trap cleanup EXIT

SERVICE_KEYS_RAW="$(yq -r '.services | keys | .[]' "$ROUTES_FILE")"
if [[ -z "${SERVICE_KEYS_RAW}" ]]; then
  echo "[error] no services found in $ROUTES_FILE (expected .services.*)" >&2
  exit 1
fi

generated=0

while IFS= read -r service_key; do
  [[ -z "$service_key" ]] && continue

  proto_file="$PROJECT_ROOT/proto/${service_key}.proto"
  valid_services_file=""
  valid_rpcs_file=""
  routes_tsv_file=""
  proto_package=""

  if [[ ! -f "$proto_file" ]]; then
    echo "[error] proto file not found for service '${service_key}': $proto_file" >&2
    exit 1
  fi

  valid_services_file="$(mktemp "${TMPDIR:-/tmp}/swagger_http.${service_key}.services.XXXXXX")"
  valid_rpcs_file="$(mktemp "${TMPDIR:-/tmp}/swagger_http.${service_key}.rpcs.XXXXXX")"
  routes_tsv_file="$(mktemp "${TMPDIR:-/tmp}/swagger_http.${service_key}.routes.XXXXXX")"
  TMP_FILES+=("$valid_services_file" "$valid_rpcs_file" "$routes_tsv_file")

  proto_package="$(grep -E '^[[:space:]]*package[[:space:]]+[A-Za-z0-9_.]+[[:space:]]*;' "$proto_file" \
    | head -n 1 \
    | sed -E 's/^[[:space:]]*package[[:space:]]+([A-Za-z0-9_.]+)[[:space:]]*;.*/\1/' || true)"
  if [[ -z "$proto_package" ]]; then
    echo "[error] failed to parse proto package from: $proto_file" >&2
    exit 1
  fi

  yq -r ".services.${service_key}.routes[] | [.method, .path, .service, .rpc, (.tag // \"\"), (.summary // \"\"), (.description // \"\")] | @tsv" "$ROUTES_FILE" \
    >"$routes_tsv_file"
  if [[ ! -s "$routes_tsv_file" ]]; then
    echo "[error] no routes found for service '${service_key}' in $ROUTES_FILE" >&2
    exit 1
  fi

  # Best-effort proto parsing; enough to catch obvious drift between routes.yaml and proto definitions.
  grep -E '^[[:space:]]*service[[:space:]]+[A-Za-z0-9_]+[[:space:]]*[{]' "$proto_file" \
    | sed -E 's/^[[:space:]]*service[[:space:]]+([A-Za-z0-9_]+)[[:space:]]*[{].*/\1/' \
    | sort -u >"$valid_services_file" || true

  grep -E '^[[:space:]]*rpc[[:space:]]+[A-Za-z0-9_]+[[:space:]]*[(]' "$proto_file" \
    | sed -E 's/^[[:space:]]*rpc[[:space:]]+([A-Za-z0-9_]+)[[:space:]]*[(].*/\1/' \
    | sort -u >"$valid_rpcs_file" || true

  tmp_http_file="$(mktemp "${TMPDIR:-/tmp}/swagger_http.${service_key}.http.XXXXXX")"
  tmp_openapi_file="$(mktemp "${TMPDIR:-/tmp}/swagger_http.${service_key}.openapi.XXXXXX")"
  TMP_FILES+=("$tmp_http_file" "$tmp_openapi_file")

  {
    echo "type: google.api.Service"
    echo "config_version: 3"
    echo "name: ${service_key}"
    echo "http:"
    echo "  rules:"
  } >"$tmp_http_file"

  {
    echo "openapiOptions:"
    echo "  method:"
  } >"$tmp_openapi_file"

  # Require tag descriptions to exist for every tag used by this service.
  tags_used_file="$(mktemp "${TMPDIR:-/tmp}/swagger_http.${service_key}.tags.XXXXXX")"
  TMP_FILES+=("$tags_used_file")
  cut -f5 "$routes_tsv_file" | sed '/^[[:space:]]*$/d' | sort -u >"$tags_used_file" || true
  if [[ ! -s "$tags_used_file" ]]; then
    echo "[error] missing route tag(s): routes under service '${service_key}' must set 'tag: ...' in $ROUTES_FILE" >&2
    exit 1
  fi
  while IFS= read -r tag_name; do
    tag_desc="$(yq -r ".services.${service_key}.tag_descriptions[\"${tag_name}\"] // \"\"" "$ROUTES_FILE")"
    if [[ -z "$tag_desc" ]]; then
      echo "[error] missing tag description: services.${service_key}.tag_descriptions[\"${tag_name}\"] (in $ROUTES_FILE)" >&2
      exit 1
    fi
  done <"$tags_used_file"

  while IFS=$'\t' read -r method path proto_service rpc tag summary description; do
    method_lc="$(printf '%s' "$method" | tr '[:upper:]' '[:lower:]')"

    case "$method_lc" in
      get|post|put|patch|delete) ;;
      *)
        echo "[error] unsupported HTTP method in routes.yaml: service=${service_key} method=${method}" >&2
        exit 1
        ;;
    esac

    if [[ -z "${path}" || -z "${proto_service}" || -z "${rpc}" ]]; then
      echo "[error] invalid route entry for service=${service_key} (path/service/rpc must be set)" >&2
      exit 1
    fi
    if [[ -z "${tag}" ]]; then
      echo "[error] invalid route entry for service=${service_key} rpc=${rpc}: missing 'tag' (set routes.yaml route tag, and services.${service_key}.tag_descriptions)" >&2
      exit 1
    fi

    if ! grep -qxF "$proto_service" "$valid_services_file"; then
      echo "[error] proto service not found: ${proto_file} missing 'service ${proto_service} { ... }' (routes.yaml service=${service_key})" >&2
      exit 1
    fi
    if ! grep -qxF "$rpc" "$valid_rpcs_file"; then
      echo "[error] proto rpc not found: ${proto_file} missing 'rpc ${rpc}(...)' (routes.yaml service=${service_key})" >&2
      exit 1
    fi

    {
      echo "    - selector: ${proto_package}.${proto_service}.${rpc}"
      echo "      ${method_lc}: ${path}"
      if [[ "$method_lc" != "get" && "$method_lc" != "delete" ]]; then
        echo "      body: \"*\""
      fi
      echo ""
    } >>"$tmp_http_file"

    {
      echo "    - method: ${proto_package}.${proto_service}.${rpc}"
      echo "      option:"
      echo "        tags:"
      echo "          - ${tag}"
      if [[ -n "${summary}" ]]; then
        esc_summary="$(printf '%s' "$summary" | sed 's/\\/\\\\/g; s/\"/\\\"/g')"
        echo "        summary: \"${esc_summary}\""
      fi
      if [[ -n "${description}" ]]; then
        esc_desc="$(printf '%s' "$description" | sed 's/\\/\\\\/g; s/\"/\\\"/g')"
        echo "        description: \"${esc_desc}\""
      fi
      echo ""
    } >>"$tmp_openapi_file"
  done <"$routes_tsv_file"

  out_http_file="$OUT_DIR/${service_key}_http.yaml"
  mv "$tmp_http_file" "$out_http_file"
  echo "[ok] generated: $out_http_file"

  out_openapi_file="$OUT_DIR/${service_key}_openapi.yaml"
  mv "$tmp_openapi_file" "$out_openapi_file"
  echo "[ok] generated: $out_openapi_file"
  generated=$((generated + 1))

  (
    cd "$PROJECT_ROOT/proto"
    protoc \
      --proto_path=. \
      --proto_path="$GW_DIR" \
      --openapiv2_out="$PROJECT_ROOT/$OUT_DIR" \
      --openapiv2_opt="grpc_api_configuration=$PROJECT_ROOT/$OUT_DIR/${service_key}_http.yaml,openapi_configuration=$PROJECT_ROOT/$OUT_DIR/${service_key}_openapi.yaml,generate_unbound_methods=false,use_go_templates=true,json_names_for_fields=false" \
      "${service_key}.proto"
  )

  swagger_json="$PROJECT_ROOT/$OUT_DIR/${service_key}.swagger.json"
  if [[ ! -f "$swagger_json" && -f "$PROJECT_ROOT/$OUT_DIR/apidocs.swagger.json" ]]; then
    mv "$PROJECT_ROOT/$OUT_DIR/apidocs.swagger.json" "$swagger_json"
  fi
  if [[ ! -f "$swagger_json" ]]; then
    echo "[error] swagger json not found after protoc for service '${service_key}': expected $swagger_json" >&2
    exit 1
  fi

  tmp_json="$(mktemp "${TMPDIR:-/tmp}/swagger_json.${service_key}.XXXXXX")"
  TMP_FILES+=("$tmp_json")
  tag_descs_json="$(yq -o=json -I=0 ".services.${service_key}.tag_descriptions // {}" "$ROUTES_FILE")"
  jq --argjson tagDescs "$tag_descs_json" '
    def wrapOp:
      if (type == "object"
          and (.responses? != null)
          and (.responses["200"]? != null)
          and (.responses["200"].schema? != null)
          and (.responses["200"].schema["$ref"]? != null)) then
        (.responses["200"].schema["$ref"] | sub("^#/definitions/";"")) as $d
        | .responses["200"].schema = {"$ref": ("#/definitions/GatewayUnifiedResponse_" + $d)}
      else
        .
      end;
    def reservedKeys:
      ["success","code","message","msg","request_id","timestamp","details"];
    def stripReserved($props):
      reduce reservedKeys[] as $k ($props; del(.[$k]));
    def defn($defs; $name):
      ($defs[$name] // {});
    def isLegacy($defs; $name):
      (defn($defs; $name).properties? // {}) as $p
      | ($p.success? != null
          or $p.code? != null
          or $p.message? != null
          or $p.msg? != null
          or $p.request_id? != null
          or $p.timestamp? != null
          or $p.details? != null);
    def shouldUnwrapData($defs; $name):
      (defn($defs; $name).properties? // {}) as $p
      | ($p.data? != null)
        and (
          isLegacy($defs; $name)
          or (( $p | keys | length) == 1)
          or ((( $p | keys | length) == 2) and ($p.pagination? != null))
        );
    def payloadDef($defs; $d):
      defn($defs; $d) as $root
      | ($root.properties? // {}) as $p
      | stripReserved($p) as $np
      | if shouldUnwrapData($defs; $d) then
          ($p.data as $dataSchema
            | ($np | del(.data)) as $extras
            | if ($dataSchema["$ref"]? != null) then
                ($dataSchema["$ref"] | sub("^#/definitions/";"")) as $dd
                | (defn($defs; $dd).properties? // {}) as $dp
                | stripReserved($dp) as $dp2
                | {type:"object", properties: ($dp2 + $extras)}
              else
                if (($extras | keys | length) == 0) then
                  $dataSchema
                else
                  {type:"object", properties: ($extras + {value: $dataSchema})}
                end
              end)
        else
          {type:"object", properties: $np}
        end;
    def wrapDef($d):
      {
        type: "object",
        properties: {
          success: {type: "boolean"},
          code: {type: "integer", format: "int32"},
          message: {type: "string"},
          data: {"$ref": ("#/definitions/GatewayPayload_" + $d)},
          request_id: {type: "string"},
          timestamp: {type: "string", format: "date-time"},
          details: {type: "object"}
        }
      };

    .schemes=["http","https"]
    | del(.host)
    | ([.paths[]? | to_entries[]? | .value.tags[]?] | unique) as $opTags
    | (.tags // []) as $existing
    | (reduce $existing[]? as $t ({}; .[$t.name]=$t)) as $existingByName
    | .tags = [
        $opTags[] as $name
        | ($existingByName[$name] // {name: $name}) as $base
        | {
            name: $name,
            description: (
              if ($base.description // "") != "" then $base.description
              else ($tagDescs[$name] // "")
              end
            )
          }
      ]
    | (.definitions // {}) as $defs
    # NOTE: API Gateway always returns the unified envelope, and "data" is the normalized payload
    # (legacy envelope fields stripped, legacy data wrapper flattened) to avoid data.data and duplicates.
    # grpc-gateway protoc-gen-openapiv2 only knows about the proto response, so we post-process the
    # swagger to make the spec match runtime payloads.
    | ([.paths[]? | to_entries[]? | .value.responses? | .["200"]?.schema? | .["$ref"]?]
        | map(select(. != null) | sub("^#/definitions/";""))
        | unique) as $respDefs
    | .definitions = $defs
      + (reduce $respDefs[] as $d ({}; .["GatewayPayload_" + $d] = payloadDef($defs; $d)))
      + (reduce $respDefs[] as $d ({}; .["GatewayUnifiedResponse_" + $d] = wrapDef($d)))
    | .paths = (.paths // {})
    | .paths |= with_entries(
        .value |= (if (type == "object") then with_entries(.value |= wrapOp) else . end)
      )
  ' "$swagger_json" >"$tmp_json"
  mv "$tmp_json" "$swagger_json"
  echo "[ok] generated: $OUT_DIR/${service_key}.swagger.json"
done <<<"$SERVICE_KEYS_RAW"

# ============================================================================
# 处理自定义端点（custom_endpoints）- 直接生成 OpenAPI 规范合并到 swagger.json
# ============================================================================

CUSTOM_SERVICES_RAW="$(yq -r '.custom_endpoints | keys | .[]' "$ROUTES_FILE" 2>/dev/null || true)"

if [[ -n "$CUSTOM_SERVICES_RAW" ]]; then
  echo "[info] Processing custom endpoints..."

  while IFS= read -r service_key; do
    [[ -z "$service_key" ]] && continue

    swagger_json="$PROJECT_ROOT/$OUT_DIR/${service_key}.swagger.json"
    if [[ ! -f "$swagger_json" ]]; then
      echo "[warn] Swagger JSON not found for service '${service_key}', skipping custom endpoints" >&2
      continue
    fi

    custom_endpoints_json="$(yq -o=json -I=0 ".custom_endpoints.${service_key} // []" "$ROUTES_FILE")"
    endpoint_count="$(echo "$custom_endpoints_json" | jq 'length')"

    if [[ "$endpoint_count" -eq 0 ]]; then
      continue
    fi

    echo "[info] Adding ${endpoint_count} custom endpoint(s) to ${service_key}.swagger.json"

    tmp_custom_json="$(mktemp "${TMPDIR:-/tmp}/swagger_custom.${service_key}.XXXXXX")"
    TMP_FILES+=("$tmp_custom_json")

    jq --argjson endpoints "$custom_endpoints_json" '
      # 添加通用错误响应定义（如果不存在）
      .definitions.ErrorResponse = (.definitions.ErrorResponse // {
        type: "object",
        properties: {
          success: {type: "boolean", example: false},
          code: {type: "integer", format: "int32"},
          message: {type: "string"},
          request_id: {type: "string"},
          timestamp: {type: "string", format: "date-time"},
          details: {type: "object"}
        }
      })
      # 处理每个自定义端点
      | reduce $endpoints[] as $ep (
        .;
        ($ep.path) as $path
        | ($ep.method | ascii_downcase) as $method
        | ($ep.tag // "Custom") as $tag
        | ($ep.summary // "") as $summary
        | ($ep.description // "") as $desc
        | ($ep.consumes // ["application/json"]) as $consumes
        | ($ep.produces // ["application/json"]) as $produces
        | ($ep.parameters // []) as $params
        | ($ep.responses // {}) as $responses

        # 确保 tag 存在于 tags 数组中
        | if ([.tags[]? | select(.name == $tag)] | length) == 0 then
            .tags += [{name: $tag, description: ""}]
          else
            .
          end

        # 添加到 paths
        | .paths[$path] = (.paths[$path] // {})
        | .paths[$path][$method] = {
            summary: $summary,
            description: $desc,
            operationId: ("Custom_" + ($path | gsub("[^a-zA-Z0-9]"; "_")) + "_" + ($method | ascii_upcase)),
            consumes: $consumes,
            produces: $produces,
            tags: [$tag],
            parameters: $params,
            responses: $responses
          }
      )
    ' "$swagger_json" > "$tmp_custom_json"

    mv "$tmp_custom_json" "$swagger_json"
    echo "[ok] Updated ${service_key}.swagger.json with custom endpoints"

  done <<<"$CUSTOM_SERVICES_RAW"
fi

echo "[done] synced ${generated} service(s) into $OUT_DIR"
