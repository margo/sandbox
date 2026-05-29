# Manual Test Cases - WFM Supplier Functional Tests

## Overview
This directory is for storing manually crafted Postman collection JSON files for WFM Supplier functional tests (MARGO template).

## Format Requirements
- Files must be valid **Postman Collection v2.1** format
- Must be in **JSON** format
- Required fields:
  - `info`: Collection metadata (with `name` and `schema` URI)
  - `item`: Array of test items (requests)
  - `variable`: (optional) Collection-level variables

## Directory Structure
```
manual-test-cases/
├── README.md                    (this file)
├── wfm-supplier/                (WFM test cases)
│   ├── postman_collection.json  (main collection)
│   └── environment.json         (optional: test environment variables)
├── device-supplier/             (Device test cases)
│   └── postman_collection.json  (main collection)
└── schemas/                     (optional: JSON schemas for validation)
    └── postman-collection-schema.json
```

## Example Usage

### Method 1: Interactive Menu
```bash
bash conformance.sh
[Select 1] WFM Supplier
[Select 2] Functional tests (in MARGO template)
[Enter path] /path/to/postman_collection.json
```

### Method 2: Direct Command
```bash
bash conformance.sh wfm functional /path/to/postman_collection.json
```

## Postman Collection Format Example
```json
{
  "info": {
    "name": "WFM Supplier Functional Tests",
    "schema": "https://schema.getpostman.com/json/collection/v2.1.0/collection.json"
  },
  "item": [
    {
      "name": "Test 1: Create Deployment",
      "request": {
        "method": "POST",
        "url": {
          "raw": "https://localhost:3001/v1alpha2/margo/deployments",
          "protocol": "https",
          "host": ["localhost"],
          "port": "3001",
          "path": ["v1alpha2", "margo", "deployments"]
        }
      }
    }
  ],
  "variable": [
    {
      "key": "base_url",
      "value": "https://localhost:3001"
    }
  ]
}
```

## Validation
When you provide a collection file, it will be validated for:
- ✓ File exists and is readable
- ✓ Valid JSON format
- ✓ Contains required Postman fields (`info` and `item`)
- ✓ Has at least one test item

## Output
Validated collections are copied to:
```
Data-Generator/wfm-supplier/postman_collection_functional.json
```

## Next Steps
After generating functional tests, use the execution CLI to run them:
```bash
bash run-tests.sh wfm
```

## Tips
- Use Postman GUI to export collections in v2.1 format
- Validate your JSON before providing the path
- Include authentication setup if required for your tests
- Use collection variables for environment-specific values
