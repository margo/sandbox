##### [Back To Main](../README.md)
# Observability Stack Verification Guide

This guide helps you verify your observability stack is working correctly.

---

## What You'll Need

- ✅ Web browser (Chrome, Firefox, Edge, or Safari)
- ✅ WFM VM IP address
- ✅ Device VM IP addresses

**How to Find Your WFM IP Address (if you don't have it):**
- It was shown during setup in Step 4 of the [Simplified-Setup-Guide](../../docs/simplified-setup-guide.md)
- It looks like: `192.168.1.100` or `10.0.0.50`
- Write it down - you'll use it throughout this guide

---

## Part 1: WFM VM Dashboard Access Verification

### Verification 1.1: Can You Access Grafana?

**What we're checking:** Grafana monitoring dashboard is running and accessible

**Steps:**

1. **Open your web browser**

2. **Type this URL** (replace `[WFM-IP]` with your actual WFM IP):
   ```
   http://[WFM-IP]:32000
   ```
   **Example:** `http://192.168.1.100:32000`

3. **What you should see:**

   | ✅ Success | ❌ Problem |
   |-----------|-----------|
   | !`[Grafana Login Page](image-placeholder)` | Browser shows "This site can't be reached" |
   | Page shows "Welcome to Grafana" | Browser shows "Connection timed out" |
   | Username and Password fields visible | Page shows "404 Not Found" |
   | Grafana logo in top-left corner | Blank white page |

4. **Try logging in:**
   - **Username:** `admin`
   - **Password:** `admin`
   - Click **Log in**

5. **After login, you should see:**
   - Grafana home page
   - Left sidebar with menu options
   - "Welcome to Grafana" message or dashboard list

**Success Criteria:**
- ✅ Page loads within 5 seconds
- ✅ Login page appears correctly
- ✅ You can log in successfully
- ✅ Home page displays after login

**If You See Problems:**

| What You See | What It Means | What to Do |
|--------------|---------------|------------|
| "This site can't be reached" | Grafana not running or wrong IP | Double-check your WFM IP address |
| "Connection timed out" | Network/firewall issue | Ask administrator to check firewall |
| "404 Not Found" | Wrong port number | Make sure you're using port `:32000` |
| Login fails with "Invalid credentials" | Wrong password | Try `admin`/`admin` again (case-sensitive) |

---

### Verification 1.2: Can You Access Prometheus?

**What we're checking:** Prometheus metrics database is running

**Steps:**

1. **Open a new browser tab**

2. **Type this URL:**
   ```
   http://[WFM-IP]:30900
   ```
   **Example:** `http://192.168.1.100:30900`

3. **What you should see:**

   | ✅ Success | ❌ Problem |
   |-----------|-----------|
   | Prometheus logo in top-left | "This site can't be reached" |
   | Blue navigation bar with "Graph", "Alerts", "Status" | "Connection refused" |
   | Search box in the middle | Blank page |
   | "Expression" input field | Error message |

4. **Visual Checklist:**
   - [ ] Prometheus logo visible (orange/red flame icon)
   - [ ] Navigation tabs: Graph, Alerts, Status
   - [ ] Search/query box in center
   - [ ] "Execute" button visible

**Success Criteria:**
- ✅ Page loads within 5 seconds
- ✅ Prometheus interface appears
- ✅ No error messages displayed
- ✅ Search box is interactive (you can click in it)

---

### Verification 1.3: Can You Access Jaeger?

**What we're checking:** Jaeger tracing system is running

**Steps:**

1. **Open a new browser tab**

2. **Type this URL:**
   ```
   http://[WFM-IP]:32500
   ```
   **Example:** `http://192.168.1.100:32500`

3. **What you should see:**

   | ✅ Success | ❌ Problem |
   |-----------|-----------|
   | "Jaeger UI" title at top | "Cannot connect" error |
   | "Search" section with dropdowns | Blank page |
   | "Service" dropdown menu | "502 Bad Gateway" |
   | "Find Traces" button | Page keeps loading forever |

4. **Visual Checklist:**
   - [ ] "Jaeger UI" text in header
   - [ ] "Service" dropdown (may show "- Select a service -")
   - [ ] "Lookback" dropdown
   - [ ] "Find Traces" button (blue)

**Success Criteria:**
- ✅ Page loads within 5 seconds
- ✅ Jaeger interface appears
- ✅ Dropdown menus are clickable
- ✅ "Find Traces" button is visible

---

## Part 2: Verify Prometheus is Collecting Metrics

### Verification 2.1: Check Prometheus Targets

**What we're checking:** Prometheus is connected to your devices and collecting data

**Steps:**

1. **In Prometheus** (`http://[WFM-IP]:30900`)

2. **Click on "Status" in the top navigation bar**

3. **Click on "Targets" from the dropdown menu**

4. **What you should see:**

   **For Each Device You Have:**

   | Column | What to Look For | ✅ Good | ❌ Bad |
   |--------|------------------|---------|--------|
   | **Endpoint** | Device IP and port | `http://192.168.1.101:30999/metrics` (K3s)<br>`http://192.168.1.102:8899/metrics` (Docker) | No devices listed |
   | **State** | Connection status | **UP** (green background) | **DOWN** (red background) |
   | **Labels** | Device information | Shows `job="otel-collector"` | Empty or missing |
   | **Last Scrape** | How recent | `2s ago`, `10s ago` | `5m ago`, `never` |
   | **Scrape Duration** | How fast | `0.5s`, `0.8s` | `5s`, `timeout` |
   | **Error** | Any problems | (empty/blank) | "context deadline exceeded" |

5. **Visual Guide:**

   **✅ Healthy Target Example:**
   ```
   Endpoint: http://192.168.1.101:30999/metrics
   State: UP (green)
   Last Scrape: 5s ago
   Scrape Duration: 0.234s
   Error: (blank)
   ```

   **❌ Unhealthy Target Example:**
   ```
   Endpoint: http://192.168.1.101:30999/metrics
   State: DOWN (red)
   Last Scrape: 2m ago
   Scrape Duration: 0s
   Error: Get "http://192.168.1.101:30999/metrics": context deadline exceeded
   ```

**Success Criteria:**
- ✅ You see at least one target listed (one per device)
- ✅ All targets show **UP** with green background
- ✅ "Last Scrape" shows recent time (< 30 seconds ago)
- ✅ "Error" column is empty for all targets

**Count Your Devices:**
- If you have 1 K3s device: You should see 1 target
- If you have 1 Docker device: You should see 1 target
- If you have both: You should see 2 targets

---

### Verification 2.2: Test Prometheus Can Query Metrics

**What we're checking:** Prometheus is actually collecting and storing metrics data

**Steps:**

1. **In Prometheus, click "Graph" in the top navigation**

2. **In the search box, type:** `up`

3. **Click the blue "Execute" button**

4. **Click the "Graph" tab** (next to "Table")

5. **What you should see:**

   | ✅ Success | ❌ Problem |
   |-----------|-----------|
   | A graph appears with lines | "No data" message |
   | Lines show value of `1` (meaning "up") | Graph is completely flat at `0` |
   | One line per device | No lines appear |
   | Graph shows recent time (last few minutes) | Graph shows old data only |

6. **Try more queries:**

   | Query to Type | What It Shows | Expected Result |
   |---------------|---------------|-----------------|
   | `up` | Which targets are up | Value of `1` for each device |
   | `container_cpu_load_average_10s` | CPU load | CPU load last 10s |
   | `container_cpu_usage_nanoseconds_total` | CPU usage | Total CPU time consumed |
   | `orders_processed_total` | Number of orders | Numbers that show total orders processed |

7. **For each query:**
   - Type the query in the search box
   - Click "Execute"
   - Click "Graph" tab
   - Look for lines on the graph

**Success Criteria:**
- ✅ `up` query shows value of `1` for each device
- ✅ Graphs display with colored lines
- ✅ Data is recent (within last 5 minutes)
- ✅ At least 3 different queries return data

**Visual Guide - What a Good Graph Looks Like:**
- **X-axis:** Shows time (last 1 hour by default)
- **Y-axis:** Shows metric values
- **Lines:** Colored lines showing data over time
- **Legend:** Below graph, shows what each line represents

---

## Part 3: Verify Grafana Data Sources

### Verification 3.1: Check Prometheus Data Source

**What we're checking:** Grafana can connect to Prometheus

**Steps:**

1. **Login to Grafana** (`http://[WFM-IP]:32000`)
   - Username: `admin`
   - Password: `admin`

2. **Look at the left sidebar** - you'll see a vertical menu with icons

3. **Navigate to Data Sources:**
   - Hover over or click the **"Connections"** icon (looks like a plug or connection symbol)
   - Click **"Data sources"** from the menu that appears

   **OR**

   - Look for a **gear/cog icon** ⚙️ (Settings)
   - Click on it
   - Select **"Data sources"**

4. **Look for "Prometheus" in the list**

5. **Click on "Prometheus"**

6. **Scroll down to the bottom of the page**

7. **Look for the "Save & test" button area**

8. **What you should see:**

   | ✅ Success | ❌ Problem |
   |-----------|-----------|
   | Green checkmark with "Successfully queried the Prometheus API" | Red X with error message |
   | "Data source is working" message | "HTTP Error Bad Gateway" |
   | URL shows `http://[WFM-IP]:30900` | URL is empty or wrong |

**Success Criteria:**
- ✅ Prometheus appears in data sources list
- ✅ Green success message visible
- ✅ No error messages
- ✅ URL field shows correct Prometheus address

---

### Verification 3.2: Check Loki Data Source

**What we're checking:** Grafana can connect to Loki for logs

**Steps:**

1. **In Grafana, navigate to Data Sources** (same as above)

2. **Look for "Loki" in the list**

3. **Click on "Loki"**

4. **Scroll to bottom**

5. **What you should see:**

   | ✅ Success | ❌ Problem |
   |-----------|-----------|
   | Green checkmark with "Data source connected and labels found" | Red X with error |
   | Shows number of labels found (e.g., "15 labels found") | "Failed to call resource" |
   | URL shows `http://[WFM-IP]:32100` | URL is empty or wrong |

---

## Part 4: Verify Metrics in Grafana

### Verification 4.1: View Metrics Using Explore

**What we're checking:** You can see actual metrics data in Grafana

**Steps:**

1. **In Grafana, look at the left sidebar**

2. **Find and click the "Explore" icon** (looks like a compass 🧭 or exploration symbol)
   - It's usually one of the top icons in the left sidebar
   - Hover over icons to see their names

3. **At the top of the Explore page, you'll see a dropdown** that says "Select data source" or shows the current data source

4. **Click the dropdown and select "Prometheus"**

5. **In the Metrics browser:**
   - You'll see a button or dropdown that says "Metric" or "Select metric"
   - Click on it to see a list of available metrics

6. **Try these metrics one by one:**

   | Metric to Select | What It Shows | What You Should See |
   |------------------|---------------|---------------------|
   | `up` | Device connectivity | Value of `1` (blue line) |
   | `system_cpu_utilization` | CPU usage | Values between 0-1, changing over time |
   | `system_memory_usage` | Memory usage | Large numbers (in bytes) |
   | `system_network_io` | Network traffic | Numbers that fluctuate |

7. **For each metric:**
   - Select the metric from the dropdown
   - Click **"Run query"** button (top right corner, blue button)
   - Look at the graph that appears below

**Success Criteria:**
- ✅ Metrics dropdown shows many options (50+ metrics)
- ✅ Selecting a metric shows a graph
- ✅ Graph has colored lines (not empty)
- ✅ Time range shows "Last 1 hour" or similar
- ✅ Data is recent (lines extend to the right edge of graph)

---

### Verification 4.2: View Custom OTEL App Metrics (If Deployed)

**What we're checking:** If you deployed the Custom OTEL application, its metrics are visible

**Prerequisites:** You must have deployed the `custom-otel-helm-app` using the WFM CLI

**Steps:**

1. **In Grafana Explore (with Prometheus selected)**

2. **In the Metrics browser, search for:** `orders_processed_total`
   - Type it in the metric search box
   - Or scroll through the dropdown to find it

3. **Select it and click "Run query"**

4. **What you should see:**

   | ✅ Success | ❌ Problem |
   |-----------|-----------|
   | Graph shows an upward trending line | "No data" message |
   | Line increases over time (orders being processed) | Metric not found in dropdown |
   | Values increase steadily | Graph is flat at zero |
   | Legend shows service name | Error message appears |

5. **Try other OTEL app metrics:**
   - `orders_processed_total` - Total orders processed
   - `http_requests_total` - HTTP requests count
   - `http_request_duration_seconds` - Request latency

**Success Criteria:**
- ✅ `orders_processed_total` metric exists
- ✅ Graph shows increasing values
- ✅ Data is being updated (line extends to current time)
- ✅ At least 2-3 OTEL app metrics are visible

---

## Part 5: Verify Logs in Grafana

### Verification 5.1: View Logs Using Explore

**What we're checking:** Grafana can display logs from your devices

**Steps:**

1. **In Grafana, click the "Explore" icon** in the left sidebar (compass icon 🧭)

2. **At the top, click the data source dropdown and select "Loki"**

3. **Build a log query:**

   | Step | Action | What to Do |
   |------|--------|------------|
   | 1 | Look for "Label filters" section | Usually below the data source dropdown |
   | 2 | Click **"+ Label filter"** or **"+ Add filter"** | Opens filter options |
   | 3 | In the first dropdown, select | `job` |
   | 4 | In the second dropdown, select | `podlogs` (K3s) or `dockerlogs` (Docker) |
   | 5 | Click **"Run query"** | Top right corner, blue button |

4. **What you should see:**

   | ✅ Success | ❌ Problem |
   |-----------|-----------|
   | Log lines appear below | "No logs found" message |
   | Timestamps on the left | Empty results area |
   | Log text is readable | "Failed to call resource" error |
   | Logs are recent (within last few minutes) | Only old logs (hours/days ago) |

**Success Criteria:**
- ✅ Log lines appear (at least 10-20 lines)
- ✅ Timestamps are recent (within last 5 minutes)
- ✅ Log text is readable and makes sense
- ✅ You can scroll through logs
- ✅ Clicking a log line expands it to show details

---

### Verification 5.2: View Custom OTEL App Logs (If Deployed)

**What we're checking:** Logs from the Custom OTEL application are visible

**Prerequisites:** Custom OTEL app must be deployed

**Steps:**

1. **In Grafana Explore (with Loki selected)**

2. **Add label filters:**
   - Click **"+ Label filter"**
   - Label: `job`
   - Operator: `=` (equals)
   - Value: `default/custom-otel-helm` (for K3s device)

3. **Click "Run query"**

4. **What you should see:**

   | ✅ Success | ❌ Problem |
   |-----------|-----------|
   | Logs from OTEL app appear | No logs found |
   | Messages about "Processing order" | Wrong application logs |
   | Recent timestamps | Only old logs |
   | Multiple log entries per second | Very few logs |

**Success Criteria:**
- ✅ OTEL app logs are visible
- ✅ Logs show application activity (orders, requests)
- ✅ Log volume is steady (new logs appearing)
- ✅ No error messages in logs

**Example OTEL App Logs:**
```
level=info msg="Order processed" order_id=12345
level=info msg="Sending trace to Jaeger"
level=debug msg="Cache updated"
```

---

### Verification 5.3: Filter and Search Logs

**What we're checking:** You can search and filter logs effectively

**Steps:**

1. **In Grafana Explore (Loki selected, with logs showing)**

2. **Look for the search/filter box** (usually above the log results)

3. **Try these filtering techniques:**

   | Filter Type | How to Do It | What It Does |
   |-------------|--------------|--------------|
   | **Search text** | Type in the search box | Shows only logs containing that text |
   | **Filter by level** | Add filter: `level="error"` | Shows only error logs |
   | **Time range** | Click time picker (top right) | Change how far back to look |
   | **Multiple filters** | Add multiple label filters | Narrow down results |

4. **Try this search:**
   - In the search/filter box, type: `error`
   - Click "Run query"
   - You should see only logs containing "error"

5. **Try changing time range:**
   - Look for the time picker in the top right (shows something like "Last 1 hour")
   - Click on it
   - Select "Last 5 minutes"
   - Click "Run query"
   - Logs should update to show only last 5 minutes

**Success Criteria:**
- ✅ Search box filters logs correctly
- ✅ Time range changes affect results
- ✅ Can combine multiple filters
- ✅ Results update when you change filters

---

## Grafana UI Quick Reference

**Left Sidebar Icons (from top to bottom, typical layout):**

| Icon | Name | What It Does |
|------|------|--------------|
| 🏠 | Home | Returns to home dashboard |
| 🧭 | Explore | Query and explore data (metrics & logs) |
| 📊 | Dashboards | View and manage dashboards |
| ⭐ | Starred | Your favorite dashboards |
| 🔔 | Alerting | Manage alerts |
| 🔌 | Connections | Data sources and plugins |
| ⚙️ | Administration/Configuration | Settings and configuration |

**Top Right Corner (on most pages):**

| Element | What It Does |
|---------|--------------|
| **Time picker** | Select time range for data (e.g., "Last 1 hour") |
| **Refresh button** | Reload current data |
| **Run query button** | Execute your query (blue button in Explore) |

---
