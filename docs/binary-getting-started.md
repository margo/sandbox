# 🚀 Device Agent – Binary Getting Started Guide

This guide explains how to setup, configure, and run the `device-agent` binary.

The `device-agent` is responsible for managing and executing workloads on a device. It applies configurations and runs workloads based on the provided setup.

---

## 📌 Overview

This guide specifically covers the **device-agent binary**.

The binary requires:
- Configuration file (`config.yaml`)  
- Certificates  

---


## ✅ Prerequisite

Ensure the following before proceeding:

- Required certificates are available  
- Ensure the required backend service (e.g., WFM, if used) is running  
- Observability stack should be available as part of the Margo ecosystem (e.g., OTEL Collector, Grafana, Jaeger, Prometheus, etc.)

👉 Note: Device-agent is not strictly dependent on the observability stack, but skipping it may result in non-compliance with Margo device requirements.


---

## 📌 Minimum Requirements

The device-agent is lightweight and can run on low-resource systems.

Approximate runtime footprint:

- CPU: ~1–2 vCPU  
- Memory: ~200–500 MB  
- Disk: ~500 MB  

👉 Note: These values are recommended for running the device-agent in a stable environment.  
If other components (e.g., WFM) are running on the same machine, additional resources should be allocated accordingly.


---

## ⚙️ Getting Started

### ✅ Generate Device Certificates (if not already available)

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

---

## 📂 Copy Certificates

```bash
cp <path-to-wfm-cert>/ca-cert.pem ./config && \
cp <path-to-your-device-certs>/* ./config
```

---

## ⚙️ Configuration

The `device-agent` uses a `config.yaml` file to define its runtime behavior.

This file typically includes:

- Certificate paths  
- Runtime configuration (Docker / Kubernetes)  
- Connection or service settings  

👉 Before running the binary, review the default configuration and update values based on your environment.

Ensure:
- Certificate paths are correct  
- Required runtime is properly configured  

---

## ▶️ Run the Binary

```bash
./device-agent --config config/config.yaml
```

---

## ✅ Verify

### 🔹 Check Running Process

```bash
ps -ef | grep device-agent
```

Example output:
```
root     7978 ... ./device-agent -config /config/config.yaml
```

👉 If the process is visible, it confirms that the device-agent is running.


## ✅ Execution Flow Summary

1. Generate certificates (if required)  
2. Copy certificates to `config/`  
3. Configure `config.yaml`  
4. Run the binary  
5. Verify using process or logs  

---

## ⚠️ Important Notes

- Ensure correct paths in `config.yaml`  
- Missing certificates will cause startup failure  
- Verify certificates before running  
- Use logs for debugging issues  

---