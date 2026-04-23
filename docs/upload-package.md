##### [Back To Main](../README.md)
### 📄 Guide to upload Application packages to Local Harbor

---

**Point-1** : App Package Naming Convention. All packages must suffixed with `-app-package`.
```
The valid App Package Naming Convention is <application-name>-<app-type>-app-package, where <application-name> is the name of the application and <app-type> is either helm or compose.
```

Examples:
- ✅ `nginx-helm-app-package`
- ✅ `wordpress-compose-app-package`
- ❌ `nginx` (missing suffix)
- ❌ `wordpress-compose` (missing `-app-package`)

**Point-2** : Ensure all artifacts **helm charts, docker images and application package itself** related to an application must be pushed to Harbor OCI .

**Point-3** : Ensure to upload all artifacts on default **library** project/repository on Harbor.

---

### Below are example commands for your reference.
> **Note** 172.19.59.148:8443 is Harbor IP and Port

**Push Images**
```bash
## Pull official Nginx image (if Not Locally present)
docker pull nginx:1.25.0

## Tag for Harbor
docker tag nginx:1.25.0 172.19.59.148:8443/library/nginx:1.25.0

## Login to Harbor
docker login 172.19.59.148:8443 -u admin -p Harbor12345

## Push to Harbor
docker push 172.19.59.148:8443/library/nginx:1.25.0
```

**Push Helm Chart**
> Ensure your helmchart Version created below is same as the **version** attribute in margo.yaml file (created as part of Margo application package).

```bash
# To push Helm Chart (. is the current directory where all helmcharts are present navigate to the directory and run below commands)
helm package .
helm push nginx-helm-1.0.0.tgz oci://172.19.59.148:8443/library --plain-http
```

**Push Application Package**
> Ensure to push package with tag **latest** as while pulling from the harbor the tag latest will be considered.
```bash
# Login to Harbor
echo "Harbor12345" | oras login 172.19.59.148:8443 \
  -u admin --password-stdin --plain-http

# Navigate to package directory where margo.yaml and /resources present and push package
oras push 172.19.59.148:8443/library/nginx-helm-app-package:latest \
  --artifact-type "application/vnd.margo.app.v1+json" \
  --plain-http \
  margo.yaml:application/vnd.margo.app.description.v1+yaml \
  resources/description.md:text/markdown \
  resources/license.md:text/markdown \
  resources/margo.jpg:image/jpeg \
  resources/release-notes.md:text/markdown
```
