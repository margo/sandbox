# 🚀 Device Agent – Binary Getting Started Guide

This guide explains how to setup, configure, and run the `device-agent` binary.

---

### ✅ Setup Note

Previously, the Margo setup required multiple VMs (separate environments for WFM and device nodes).  

✅ With the `device-agent` binary, you can now run both:
- WFM (Workload Fleet Manager)
- Device Agent  

👉 on a **single VM / machine**.

This simplifies the setup and reduces resource requirements.

---
### 📘 Reference Setup Guide
For detailed environment setup and WFM installation (non-binary / full setup), refer to:
👉 [Margo Setup Guide](./setup-guide.md)

---



## ✅ Make Binary Executable

```bash
chmod +x device-agent
```
---
### ✅ Prerequisite
- WFM (Workload Fleet Manager) should be running

---
## 📌 Overview

The `device-agent` binary requires:
- Configuration file  
- Symphony certificates  
- Device certificates  

Before running the binary, ensure all required certificates are generated and placed correctly.

---

## ⚙️ Getting Started

### 1. Generate Certificates (If Not Already Available)

If certificates are **not already generated**, generate them first.



### ✅ Generate Device Certificates

#### 🔹 RSA (Recommended)

```bash
mkdir -p $HOME/certs
cd $HOME/certs

openssl genrsa -out device-private.key 2048

openssl req -new -x509 \
  -key device-private.key \
  -out device-public.crt \
  -days 365 \
  -subj "/C=IN/ST=GGN/L=Sector 48/O=Margo/CN=margo-device"
```

✅ Generates:
- device-private.key  
- device-public.crt  

---

#### 🔹 ECDSA (Alternative)

```bash
mkdir -p $HOME/certs
cd $HOME/certs

openssl ecparam -genkey -name prime256v1 -out device-ecdsa.key

openssl req -new -x509 \
  -key device-ecdsa.key \
  -out device-ecdsa.crt \
  -days 365 \
  -subj "/C=IN/ST=GGN/L=Sector 48/O=Margo/CN=margo-device"
```

✅ Generates:
- device-ecdsa.key  
- device-ecdsa.crt  

---

⚠️ Ensure both WFM and Device certificates are generated before proceeding.

---

## 📂 2. Copy Certificates to Config Directory

### ✅ Copy Symphony CA Certificate

```bash
cp <path-to-wfm-cert>/ca-cert.pem ./config
```

### ✅ Copy Client Certificates

```bash
cp <path-to-your-device-certs>/* ./config
```

---

---

> 💡 You can configure either the Kubernetes (k3s/helm) runtime or the Docker runtime based on your setup. To switch between runtimes, you will need to update the `config.yaml` file by enabling the required runtime and commenting out the other. Only one runtime should be active at a time.

---

## ▶️ 4. Run the Binary

```bash
./device-agent --config config.yaml
```

---

## ✅ Verify

```bash
~./device-agent --help
```


## ✅ Execution Flow Summary

1. Generate certificates (if not present)  
2. Copy certificates to config folder  
3. Build binary (if needed)  
4. Run binary  

---

## ⚠️ Important Notes

- Ensure correct file paths in config.yaml  
- Missing certificates will cause startup failure  
- Verify certificates before running  
- Check logs if binary fails to start  

---