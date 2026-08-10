# Margo Conformance Testing - Documentation Index

**Complete guide to understanding and running Margo conformance tests**

---

## 📖 Documentation Guide

### 🚀 **Start Here: Quick-Start Guide**
- **File:** [quick-start.md](./quick-start.md)
- **For:** Users who want to run tests immediately
- **Contains:** 
  - Installation steps
  - Quick commands for both WFM and Device testing
  - One-line execution examples
  - Troubleshooting tips
  - File locations
- **Read time:** 5-10 minutes
- **Best for:** First-time users, getting tests running quickly

---

### 📚 **Deep Dive: Complete System Overview**
- **File:** [summary.md](./summary.md)
- **For:** Users who want to understand how everything works
- **Contains:**
  - What is Margo? (High-level explanation)
  - Persona roles & responsibilities (WFM Supplier vs Device Supplier)
  - Two-CLI architecture explanation
  - End-to-end workflows with examples
  - Detailed WFM Supplier testing process
  - Detailed Device Supplier testing process
  - System components and communication patterns
  - Key concepts (OpenAPI, RFC 9421, Context Chaining, etc.)
  - Test data flow through the system
  - Environment setup for multi-machine deployments
- **Read time:** 30-45 minutes
- **Best for:** Understanding the full ecosystem, explaining to others, technical decision-making
- **Reading path:**
  1. What is Margo?
  2. Personas & Their Roles
  3. Two-CLI Architecture
  4. Your specific workflow (WFM or Device)
  5. Key Concepts (for deeper understanding)

---

### 🛠️ **Setup & Configuration**

#### Environment Setup
- **File:** [env-setup.md](./env-setup.md)
- **For:** Setting up environment variables
- **Contains:** Variable configuration for WFM and Device agents
- **When needed:** Before running multi-machine deployments

#### Setup Guide
- **File:** [setup-guide.md](./setup-guide.md)
- **For:** Full environment initialization from scratch
- **Contains:**
  - VM requirements (3 machines, specs)
  - Network configuration
  - WFM installation and startup
  - Device agent installation
  - Monitoring tools setup
  - Verification steps
- **When needed:** First-time environment setup
- **Read time:** Reference document (follow step-by-step)

---

### 🔧 **Development & Architecture**

#### Development Toolsets
- **File:** [dev-toolsets.md](./dev-toolsets.md)
- **For:** Understanding tools used in the system
- **Contains:**
  - Core tools: Go, Docker, Rust, Kubernetes
  - Container orchestration: K3s, Helm
  - Registry management: Harbor
  - Observability: Prometheus, Grafana, Jaeger, Loki, OTEL
  - Versions and purposes
- **When needed:** Understanding system architecture, troubleshooting tool issues

#### Repository Structure
- **File:** [repo-structure.md](./repo-structure.md)
- **For:** Understanding file organization
- **Contains:** Directory layout and purpose of each folder
- **When needed:** Navigating the codebase

---

### 📊 **Visual Architecture Diagrams**

#### Margo Architecture
- **File:** [margo-architecture.png](./margo-architecture.png)
- **Shows:** High-level Margo ecosystem with 3 personas
- **Components:** WFM, Devices, Applications, Registry, Observability
- **View:** Visual overview of how Margo components interact

#### Overlay Architecture
- **File:** [overlay-architecture.png](./overlay-architecture.png)
- **Shows:** Deployment options and integration patterns
- **Components:** WFM, Device clusters, standalone devices, observability
- **View:** How different Margo deployments can be configured

---

## 🎯 Learning Paths

### Path 1: "I just want to run tests" (15 minutes)
```
1. Read: quick-start.md (5 min)
   → Get basic understanding

2. Run: One of the quick commands
   $ bash conformance.sh wfm openapi "spec-url"
   $ bash run-tests.sh wfm

3. View: HTML report in browser
   → See test results
```

### Path 2: "I want to understand the system" (1 hour)
```
1. Read: What is Margo? (summary.md)
   → Understand the problem Margo solves

2. Read: Personas & Their Roles (summary.md)
   → Learn about WFM Supplier and Device Supplier

3. Read: Two-CLI Architecture (summary.md)
   → Understand preparation vs execution

4. Read: Your specific workflow
   → Deep dive into WFM or Device testing

5. Optional: Key Concepts (summary.md)
   → RFC 9421, OpenAPI, Context Chaining
```

### Path 3: "I'm setting up a multi-machine environment" (2+ hours)
```
1. Read: setup-guide.md
   → Understand requirements and VM setup

2. Read: env-setup.md
   → Configure environment variables

3. Follow: Step-by-step setup instructions
   → Install WFM, Devices, Test tools

4. Run: Conformance tests
   → Validate everything works

5. Refer: dev-toolsets.md if tools need adjustment
```

### Path 4: "I'm a developer troubleshooting issues" (30+ minutes)
```
1. Check: quick-start.md troubleshooting section
   → Common errors and solutions

2. Read: relevant section in summary.md
   → Deep understanding of failing component

3. Check: dev-toolsets.md
   → Tool versions and configuration

4. Review: setup-guide.md
   → Verify environment is correct

5. Check: repo-structure.md
   → Navigate to relevant code files
```

---

## 🔑 Key Concepts Quick Reference

| Concept | File | Section |
|---------|------|---------|
| **What is Margo?** | summary.md | What is Margo? |
| **WFM Supplier Testing** | summary.md | WFM Supplier Workflow |
| **Device Supplier Testing** | summary.md | Device Supplier Workflow |
| **Two-CLI Architecture** | summary.md | Two-CLI Architecture |
| **RFC 9421 Signatures** | summary.md | Key Concepts |
| **OpenAPI Specifications** | summary.md | Key Concepts |
| **Context Chaining** | summary.md | Key Concepts |
| **Quick Commands** | quick-start.md | Quick Commands |
| **Troubleshooting** | quick-start.md | Troubleshooting |
| **Tools & Versions** | dev-toolsets.md | Development Toolsets |
| **Environment Variables** | env-setup.md | Environment Setup |

---

## 📁 File Organization

```
docs/
├── README.md (you are here)
│
├── QUICK REFERENCE
│   └── quick-start.md           (5-10 min read, how to run tests)
│
├── COMPREHENSIVE GUIDES
│   ├── summary.md               (30-45 min read, full overview)
│   ├── setup-guide.md           (reference, environment setup)
│   └── env-setup.md             (reference, configuration)
│
├── TECHNICAL DETAILS
│   ├── dev-toolsets.md          (tools and versions)
│   └── repo-structure.md        (file organization)
│
└── DIAGRAMS
    ├── margo-architecture.png   (ecosystem overview)
    └── overlay-architecture.png (deployment patterns)
```

---

## ✅ Documentation Checklist

- ✅ **Quick-Start Guide:** For running tests immediately
- ✅ **Comprehensive Summary:** For understanding everything
- ✅ **Setup Guide:** For environment initialization
- ✅ **Configuration Guide:** For environment variables
- ✅ **Tool Reference:** For understanding tech stack
- ✅ **Architecture Diagrams:** For visual understanding
- ✅ **This Index:** For navigation

---

## 🚀 Getting Started Now

**Option A: I want to run tests immediately**
1. Open: [quick-start.md](./quick-start.md)
2. Follow: Interactive menu or quick commands
3. Done! ✅

**Option B: I want to understand before running**
1. Open: [summary.md](./summary.md)
2. Read sections in this order:
   - What is Margo?
   - Personas & Their Roles
   - Two-CLI Architecture
   - Your workflow (WFM or Device)
3. Then: Follow [quick-start.md](./quick-start.md) to run tests

**Option C: I'm setting up the environment**
1. Open: [setup-guide.md](./setup-guide.md)
2. Follow: Step-by-step instructions
3. Configure: [env-setup.md](./env-setup.md)
4. Then: Run tests with [quick-start.md](./quick-start.md)

---

## 💡 Pro Tips

- **Bookmark [summary.md](./summary.md):** Refer to it frequently for understanding workflows
- **Use [quick-start.md](./quick-start.md):** As your command reference sheet
- **Read Key Concepts:** Before running tests for the first time
- **Check architecture diagrams:** When explaining to non-technical stakeholders
- **Save troubleshooting section:** For quick problem-solving

---

## 📞 Help & Support

**Having issues?**
1. Check: Troubleshooting section in [quick-start.md](./quick-start.md)
2. Read: Relevant section in [summary.md](./summary.md) for deep understanding
3. Verify: [setup-guide.md](./setup-guide.md) for environment correctness
4. Review: Log files in `Data-Generator/` and `Runner/` directories

**Want to contribute docs?**
- Follow the same structure
- Use markdown formatting
- Include examples and code blocks
- Add to this index

---

**Last Updated:** May 28, 2026  
**Version:** 1.0 - Consolidated & Simplified  
**Status:** ✅ Ready for use
