# Multi-VM Deployment Configuration Guide

**Status:** Documentation for conformance testing across multiple VMs  
**Created:** April 6, 2026

## Overview

When testing device-agents on **different VMs** against mock-WFM, the `packageLocation` field in deployment manifests must be accessible from the device-agent's VM. This guide documents multiple strategies for configuring `packageLocation` to work in multi-VM environments.

## Problem

The original hardcoded path:
```yaml
packageLocation: /var/lib/workload-deployments/docker-compose-sample.yaml
```

This path **only works locally** on the mock-WFM server VM. When device-agents run on different VMs, they cannot access this path.

---

## Solutions for Multi-VM Conformance Testing

### Strategy 1: Relative Paths with Local File Access ✅ **Recommended for Same Network**

**When to use:** Device-agents and mock-WFM share a mounted filesystem (NFS, SMB) or the same host

**Configuration:**
```yaml
packageLocation: ./docker-compose-sample.yaml
```

or

```yaml
packageLocation: ../../docker-compose/docker-compose-sample.yaml
```

**Setup:**
- Mount the compose file directory on both VMs at the same relative path
- Device-agent resolves the path relative to its working directory

**Advantages:**
- Simple, no additional infrastructure
- Works with Docker volumes and Kubernetes volume mounts
- Good for development and testing

**Example with Docker Compose:**
```bash
# On mock-WFM server
docker run -v /home/margo/nitin/sandbox/docker-compose:/var/compose mock-wfm

# Device-agent config references
packageLocation: /var/compose/docker-compose-sample.yaml
```

---

### Strategy 2: HTTP/HTTPS URLs ✅ **Best for Different Networks**

**When to use:** Device-agents cannot reach the file via shared filesystem; need maximum flexibility

**Option A: Use Public GitHub/Registry URL**

```yaml
packageLocation: https://raw.githubusercontent.com/your-org/your-repo/main/docker-compose-sample.yaml
```

**Configuration in mock-WFM deployment catalog:**
```yaml
# ✅ GOOD: Public, always available, versioned
packageLocation: https://raw.githubusercontent.com/docker/awesome-compose/refs/heads/master/nginx-flask-mysql/compose.yaml
```

**Advantages:**
- Works from any network
- Versioned and immutable (with proper tagging)
- No infrastructure required on mock-WFM server
- Can serve different versions of compose files

**Setup Steps:**
1. Commit compose file to a GitHub repository
2. Reference the raw content URL in deployment manifest
3. Device-agent downloads file via HTTP

---

**Option B: Local HTTP Server on Mock-WFM VM**

**Configuration:**
```yaml
packageLocation: http://mock-wfm-server-ip:8080/compose/docker-compose-sample.yaml
```

**Setup Steps:**

1. **Start an HTTP server** on mock-WFM VM (alongside mock-WFM service):
```bash
# Option 1: Using Python
cd /home/margo/nitin/sandbox/docker-compose
python3 -m http.server 8080

# Option 2: Using nginx
docker run -d -p 8080:80 -v /home/margo/nitin/sandbox/docker-compose:/usr/share/nginx/html nginx
```

2. **Update deployment manifest** with server IP:
```yaml
packageLocation: http://192.168.1.100:8080/docker-compose-sample.yaml
```

3. **Configure device-agent to use correct IP:**
   - Device-agent must be able to resolve `mock-wfm-server-ip`
   - Update network DNS or use `/etc/hosts` mapping

**Advantages:**
- No external dependencies
- Can serve multiple files
- Good debug feedback (HTTP logs)
- Easy to update files without redeploying

**Example with docker-compose:**
See [docker-compose.yaml](../../docker-compose/docker-compose.yaml) for complete setup

---

### Strategy 3: Environment-Based Configuration ✅ **Most Flexible**

**When to use:** Need to support multiple environments (dev, staging, prod) with different paths

**Implementation in mock-WFM:**

Update `deployment_catalog.go` to support environment variables:
```go
// In deployments loaded from mock-data/deployments/
// Replace packageLocation placeholders with environment variables
func substituteEnvVariables(yaml []byte) []byte {
    text := string(yaml)
    
    // Replace patterns like ${COMPOSE_SERVER_URL}
    for _, env := range []string{"COMPOSE_SERVER_URL", "DEPLOYMENT_BASE_PATH"} {
        if val := os.Getenv(env); val != "" {
            text = strings.ReplaceAll(text, "${"+env+"}", val)
        }
    }
    
    return []byte(text)
}
```

**Deployment manifest template:**
```yaml
# Use ${COMPOSE_SERVER_URL} placeholder
packageLocation: ${COMPOSE_SERVER_URL}/docker-compose-sample.yaml
```

**Usage:**
```bash
# Start mock-WFM with environment variables
export COMPOSE_SERVER_URL="http://192.168.1.100:8080/compose"
./mock-wfm

# Or for GitHub
export COMPOSE_SERVER_URL="https://raw.githubusercontent.com/margo/conformance/main"
./mock-wfm
```

**Device-agent experience:**
- Gets deployment from mock-WFM: `packageLocation: http://192.168.1.100:8080/compose/docker-compose-sample.yaml`
- Downloads compose file from that URL

---

## Configuration Decision Matrix

| Scenario | Recommended Strategy | Pros | Cons |
|----------|----------------------|------|------|
| **Same VM / Docker host** | Relative path or volume mount | Simple, fast | No network testing |
| **Same network, file access** | NFS/SMB mount + relative path | Native filesystem performance | Requires infrastructure |
| **Different networks** | HTTP URL (GitHub or local server) | Works anywhere, versioned | Requires network setup |
| **Multi-tenant testing** | Environment variables | Highly flexible, scalable | More complex config |
| **Public conformance suite** | Public GitHub URL | Discoverable, reproducible | External dependency |

---

## Quick Reference: Current Configuration

**Current default in `compose-sample.yaml`:**
```yaml
packageLocation: ./docker-compose-sample.yaml
```

This is **relative path approach** (Strategy 1).

**To use for multi-VM testing:**

1. **Option A: Setup shared mount**
   ```bash
   # On mock-WFM VM
   export BIND_PATH="/home/margo/nitin/sandbox/docker-compose"
   
   # On device-agent VM, mount the same location
   mount -t nfs mock-wfm-vm:$BIND_PATH /var/compose
   ```

2. **Option B: Setup HTTP server**
   ```bash
   # Update packageLocation to:
   # http://<mock-wfm-ip>:8080/docker-compose-sample.yaml
   
   # Start server on mock-WFM VM
   cd /home/margo/nitin/sandbox/docker-compose
   python3 -m http.server 8080 --bind 0.0.0.0
   ```

---

## Testing Your Configuration

### 1. Verify packageLocation is Accessible

```bash
# From device-agent VM
# For relative paths
ls -l ./docker-compose-sample.yaml

# For HTTP URLs
curl -I http://mock-wfm-ip:8080/docker-compose-sample.yaml

# For GitHub URLs
curl -I https://raw.githubusercontent.com/your-org/your-repo/main/docker-compose-sample.yaml
```

### 2. Use Postman Collection to Verify

1. **Get deployments** for a registered device:
   ```
   GET /api/v1/clients/{clientId}/deployments
   ```

2. **Verify response contains correct packageLocation**:
   ```json
   {
     "deployments": [
       {
         "deploymentProfile": {
           "components": [
             {
               "properties": {
                 "packageLocation": "http://mock-wfm-ip:8080/docker-compose-sample.yaml"
               }
             }
           ]
         }
       }
     ]
   }
   ```

3. **On device-agent VM, verify it can fetch the compose file**:
   ```bash
   # For HTTP URLs
   curl -O http://mock-wfm-ip:8080/docker-compose-sample.yaml
   
   # Verify the file is valid compose format
   docker-compose -f docker-compose-sample.yaml config
   ```

### 3. Test Full Deployment Workflow

```bash
# From mock-WFM server
./comprehensive_tests.sh

# Expected: All 39 tests pass, including deployment retrieval tests
```

---

## Troubleshooting

| Issue | Cause | Solution |
|-------|-------|----------|
| Device-agent can't find compose file | Invalid packageLocation path | Verify path matches one of the strategies above |
| HTTP 404 on packageLocation URL | File not hosted or wrong path | Check HTTP server is running and file is served |
| Relative path not found | Working directory mismatch | Update packageLocation or set correct working directory |
| Permission denied (local path) | File ownership or permissions | Fix file permissions: `chmod 644 docker-compose-sample.yaml` |
| Network timeout | Device-agent can't reach server | Verify network connectivity: `ping mock-wfm-ip` |

---

## Best Practices for Conformance Testing

1. **Use HTTP URLs with GitHub** for public conformance suites
   - Versions are immutable
   - Works from any network
   - Easy to share with other teams

2. **Document the expected packageLocation format** in your conformance test spec
   - Include examples for different deployment strategies
   - Specify whether relative or absolute paths are required

3. **Test with multiple device-agents** on different VMs
   - Ensure packageLocation works across networks
   - Validate device-agent can download and parse compose files

4. **Use Postman collection** to validate deployments before device-agent integration
   - Verify deployments are returned correctly
   - Check packageLocation values in responses

5. **Version your compose files** with git tags
   - Reference specific versions in deployments
   - Makes testing reproducible and verifiable

---

## See Also

- [DEVICE_AGENT_INTEGRATION.md](./DEVICE_AGENT_INTEGRATION.md) - Multi-VM HTTPS setup
- [POSTMAN_COLLECTION_SETUP_GUIDE.md](../../POSTMAN_COLLECTION_SETUP_GUIDE.md) - Postman testing
- [compose-sample.yaml](./mock-data/deployments/compose-sample.yaml) - Example deployment
