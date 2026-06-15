def set_json_body($raw):
  .request.body = {"mode":"raw","raw":$raw,"options":{"raw":{"language":"json"}}};
  
def patch_url_variables:
  if (.request.url.variable | type) == "array" then
    .request.url.variable |= map(
      if .key == "clientId" then . + {"value": "{{clientId}}"}
      elif .key == "deploymentId" then . + {"value": "{{deploymentId}}"}
      elif .key == "digest" then . + {"value": "{{digest}}"}
      elif .key == "bundleDigest" then . + {"value": "{{bundleDigest}}"}
      elif .key == "deploymentDigest" then . + {"value": "{{deploymentDigest}}"}
      else . end)
  else . end;

def add_flexible_test_script:
  .event = ((.event // []) | map(select(.listen != "test"))) + [{
    "listen":"test",
    "script":{"type":"text/javascript","exec":[
      "if (pm.response.code >= 200 && pm.response.code < 300) {",
      "  tests[\"Success (\" + pm.response.code + \")\"] = true;",
      "} else if (pm.response.code >= 400 && pm.response.code < 500) {",
      "  tests[\"Client Error (\" + pm.response.code + \")\"] = true;",
      "} else if (pm.response.code >= 500) {",
      "  tests[\"Server Error (\" + pm.response.code + \")\"] = true;",
      "} else {",
      "  tests[\"Request completed (\" + pm.response.code + \")\"] = true;",
      "}"
    ]}}
  ];

def patch_request:
  if (has("request") | not) then .
  else
    patch_url_variables |
    if (.request.url.path | type) == "array" then
      .request.url.path |= map(if startswith(":") then "{{" + .[1:] + "}}" else . end)
    else . end |
    ((.request.url.path // []) | join("/")) as $path |
    (.request.method // "") as $method |
    if ($method == "GET" and ($path | test("api/v1/clients/.*/bundles/"))) then add_flexible_test_script
    elif ($method == "GET" and ($path | test("api/v1/clients/.*/deployments$"))) then add_flexible_test_script
    elif ($method == "GET" and ($path | test("api/v1/clients/.*/deployments/.*/"))) then add_flexible_test_script
    elif ($method == "POST" and ($path | test("api/v1/onboarding$"))) then set_json_body("{{onboardingRequest}}") | add_flexible_test_script
    elif ($method == "POST" and ($path | test("api/v1/clients/.*/capabilities$"))) then set_json_body("{{capabilitiesRequest}}") | add_flexible_test_script
    elif ($method == "PUT" and ($path | test("api/v1/clients/.*/capabilities$"))) then set_json_body("{{capabilitiesUpdateRequest}}") | add_flexible_test_script
    elif ($method == "POST" and ($path | test("api/v1/clients/.*/deployments/.*/status$"))) then set_json_body("{{statusRequest}}") | add_flexible_test_script
    else . end
  end;

def patch_items:
  if has("item") then .item |= map(patch_items) else patch_request end;

patch_items
