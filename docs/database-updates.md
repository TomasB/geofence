# GeoFence Database Updates - DevOps Guide

## Overview

The GeoFence service uses MaxMind's GeoLite2 Country database for IP geolocation. This document describes the automated database update mechanism and operational procedures for DevOps teams.

**Key Features:**
- **Hot-reload with zero downtime**: Service automatically detects and loads new database files without restart
- **Automated daily updates**: CronJob downloads fresh MMDB data from MaxMind
- **Graceful error handling**: Failed updates don't impact running service; old database remains active

---

## Architecture

```
┌─────────────────────────────────────────────────┐
│  Kubernetes Cluster                             │
│                                                 │
│  ┌──────────────────┐      ┌─────────────────┐  │
│  │  CronJob         │      │  Deployment     │  │
│  │  (Daily 3AM UTC) │      │  (3 replicas)   │  │
│  │                  │      │                 │  │
│  │  ┌────────────┐  │      │  ┌───────────┐  │  │
│  │  │geoipupdate │  │      │  │ geofence  │  │  │
│  │  │ container  │  │      │  │    pods   │  │  │
│  │  └─────┬──────┘  │      │  └─────┬─────┘  │  │
│  └────────┼─────────┘      └────────┼────────┘  │
│           │                         │           │
│           │     writes              │  reads    │
│           ▼                         ▼           │
│  ┌───────────────────────────────────────────┐  │
│  │  PersistentVolume (ReadWriteMany)         │  │
│  │  /data/GeoLite2-Country.mmdb              │  │
│  └───────────────────────────────────────────┘  │
│                                                 │
│  Service watches file for changes using         │
│  fsnotify → atomic reload on detect             │
└─────────────────────────────────────────────────┘
```

---

## Prerequisites

### 1. MaxMind Account Setup

1. **Create MaxMind Account**
   - Visit: https://www.maxmind.com/en/geolite2/signup
   - Sign up for a free GeoLite2 account

2. **Generate License Key**
   - Log in to: https://www.maxmind.com/en/account/login
   - Navigate to: Account → License Keys
   - Click "Generate new license key"
   - **Name**: `geofence-k8s-production`
   - **Confirmation**: Check "No" for older geoipupdate versions (we use v6.0+)
   - **IMPORTANT**: Copy the license key immediately (shown only once)

3. **Record Credentials**
   - Note your **Account ID** (visible on account page)
   - Save your **License Key** securely (1Password, Vault, etc.)

### 2. Kubernetes Cluster Requirements

- Kubernetes 1.20+ with persistent volume support
- Storage class supporting `ReadWriteMany` access mode (NFS, CephFS, Azure Files, etc.)
- kubectl configured with cluster admin access

---

## Initial Deployment

### Step 1: Create Kubernetes Secret (External Secrets Management)

MaxMind credentials must be stored securely in Kubernetes. Use one of these approaches:

**Option 1: External Secrets Operator** (Recommended)
```bash
# Install External Secrets Operator
helm repo add external-secrets https://charts.external-secrets.io
helm install external-secrets \
  external-secrets/external-secrets \
  -n external-secrets-system --create-namespace

# Reference your secret backend (Vault, AWS Secrets Manager, etc.)
# See: https://external-secrets.io/latest/introduction/
```

**Option 2: Sealed Secrets**
```bash
# Install Sealed Secrets controller
kubectl apply -f https://github.com/bitnami-labs/sealed-secrets/releases/download/v0.24.0/controller.yaml

# Encrypt your secret locally, then commit sealed secret to git
echo 'MAXMIND_ACCOUNT_ID: 123456\nMAXMIND_LICENSE_KEY: yourkey' | \
  kubeseal -f - -w sealed-secret.yaml
```

**Option 3: Create Secret in Cluster** (Development Only)
```bash
# Create secret imperatively (not recommended for production)
kubectl create secret generic geofence-secret \
  --from-literal=MAXMIND_ACCOUNT_ID="123456" \
  --from-literal=MAXMIND_LICENSE_KEY="yourkey" \
  -n <namespace>
```

**Verify secret creation:**
```bash
kubectl get secret geofence-secret -n <namespace>
```

### Step 2: Deploy Database Update Infrastructure

```bash
# Deploy PVC and CronJob for database updates
kubectl apply -f cronjob.yaml -n <namespace>

# Verify PVC creation
kubectl get pvc geofence-mmdb-pvc -n <namespace>

# Verify CronJob creation
kubectl get cronjob geofence-mmdb-updater -n <namespace>
```

### Step 3: Trigger Initial Database Download

Since the CronJob runs daily at 3 AM UTC, manually trigger the first run:

```bash
# Create a one-time job from the CronJob
kubectl create job geofence-mmdb-initial \
  --from=cronjob/geofence-mmdb-updater \
  -n <namespace>

# Monitor job execution
kubectl get jobs -n <namespace> -w

# Check logs
kubectl logs job/geofence-mmdb-initial -n <namespace>
```

Expected output should include:
```
database updates available for GeoLite2-Country
```

### Step 4: Deploy GeoFence Service

```bash
# Apply ConfigMap
kubectl apply -f configmap.yaml -n <namespace>

# Deploy the service
kubectl apply -f deployment.yaml -n <namespace>

# Deploy the service endpoint
kubectl apply -f service.yaml -n <namespace>

# Verify deployment
kubectl get pods -n <namespace> -l app=geofence
kubectl get svc geofence -n <namespace>
```

### Step 5: Verify Service Health

```bash
# Port-forward to a pod
kubectl port-forward -n <namespace> svc/geofence 8080:8080

# Check health endpoint
curl http://localhost:8080/health
# Expected: {"status":"ok"}

# Check readiness (verifies MMDB loaded)
curl http://localhost:8080/ready
# Expected: {"status":"ready"}

# Test geolocation lookup
curl -X POST http://localhost:8080/api/v1/check \
  -H "Content-Type: application/json" \
  -d '{"ip":"8.8.8.8","allowed_countries":["US"]}'
# Expected: {"allowed":true,"country":"US","error":""}
```

---

## Verification Procedures

### Daily Database Update Verification

**Automated Check**: The service automatically reloads when new database files are detected.

**Manual Verification**:

```bash
# 1. Check CronJob execution history
kubectl get jobs -n <namespace> -l app=geofence-mmdb-updater

# 2. View logs from latest job
LATEST_JOB=$(kubectl get jobs -n <namespace> \
  -l app=geofence-mmdb-updater \
  --sort-by=.metadata.creationTimestamp \
  -o jsonpath='{.items[-1].metadata.name}')

kubectl logs job/$LATEST_JOB -n <namespace>

# 3. Check MMDB file timestamp in PVC
kubectl exec -n <namespace> deployment/geofence -c geofence -- \
  ls -lh /data/GeoLite2-Country.mmdb

# 4. Verify service detected the update (check application logs)
kubectl logs -n <namespace> deployment/geofence -c geofence --tail=50 | \
  grep -i "reload"
```

**Expected Log Messages**:
```
level=INFO msg="MMDB file modified, reloading database"
level=INFO msg="Successfully reloaded MMDB database"
```

### Health Check Monitoring

Set up monitoring alerts for these conditions:

1. **Readiness Probe Failures**
   ```bash
   # Check pod readiness status
   kubectl get pods -n <namespace> -l app=geofence \
     -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.status.conditions[?(@.type=="Ready")].status}{"\n"}{end}'
   ```

2. **CronJob Failures**
   ```bash
   # List failed jobs
   kubectl get jobs -n <namespace> \
     -l app=geofence-mmdb-updater \
     --field-selector status.successful=0
   ```

3. **Application Error Logs**
   ```bash
   # Search for reload errors
   kubectl logs -n <namespace> -l app=geofence --tail=100 | \
     grep -i "error.*reload"
   ```

---

## Troubleshooting Guide

### Issue: CronJob Fails to Download Database

**Symptoms:**
- CronJob shows failed status
- Logs indicate authentication or network errors

**Diagnosis:**
```bash
# Check job status
kubectl describe job <job-name> -n <namespace>

# Review logs
kubectl logs job/<job-name> -n <namespace>
```

**Common Causes & Solutions:**

1. **Invalid MaxMind Credentials**
   ```
   Error: Invalid account ID or license key
   ```
   
   **Solution:**
   ```bash
   # Verify secret contents
   kubectl get secret geofence-secret -n <namespace> -o yaml
   
   # Update credentials if needed
   kubectl delete secret geofence-secret -n <namespace>
   kubectl apply -f secret.yaml -n <namespace>
   
   # Trigger manual job to test
   kubectl create job test-credentials \
     --from=cronjob/geofence-mmdb-updater -n <namespace>
   ```

2. **Network/Firewall Issues**
   ```
   Error: Failed to connect to MaxMind servers
   ```
   
   **Solution:**
   - Verify cluster can reach `updates.maxmind.com:443`
   - Check network policies and firewall rules
   - Ensure egress traffic is allowed

3. **PVC Mount Issues**
   ```
   Error: Permission denied writing to /data
   ```
   
   **Solution:**
   ```bash
   # Check PVC status
   kubectl describe pvc geofence-mmdb-pvc -n <namespace>
   
   # Verify storage class supports ReadWriteMany
   kubectl get storageclass
   ```

### Issue: Service Not Detecting Database Updates

**Symptoms:**
- CronJob succeeds but service doesn't reload
- No reload messages in application logs

**Diagnosis:**
```bash
# Check if file watcher is active (look for startup message)
kubectl logs -n <namespace> deployment/geofence --tail=200 | \
  grep -i "watcher"

# Expected: "Started file watcher for MMDB hot-reload"
```

**Solutions:**

1. **File Watcher Disabled** (warning at startup)
   - File watcher failed to initialize but service continues
   - Manual pod restart required after database updates
   - Check logs for watcher initialization errors

2. **File System Write Method**
   - Some storage backends don't trigger fsnotify events
   - Consider restarting pods after updates if hot-reload unreliable

3. **Restart Pods Manually**
   ```bash
   # Rolling restart after confirming new database present
   kubectl rollout restart deployment/geofence -n <namespace>
   
   # Monitor rollout
   kubectl rollout status deployment/geofence -n <namespace>
   ```

### Issue: Corrupted Database File

**Symptoms:**
- Readiness probe fails (503 on `/ready`)
- Error logs: "Failed to reload MMDB"
- Service keeps old database active (graceful degradation)

**Diagnosis:**
```bash
# Check readiness probe
kubectl get pods -n <namespace> -l app=geofence

# View error details
kubectl logs -n <namespace> -l app=geofence --tail=50 | \
  grep -i "mmdb.*error"
```

**Solution:**
The service automatically retains the previous working database. To recover:

1. **Trigger fresh download:**
   ```bash
   # Delete suspicious MMDB file
   kubectl exec -n <namespace> deployment/geofence -c geofence -- \
     rm -f /data/GeoLite2-Country.mmdb
   
   # Trigger CronJob manually
   kubectl create job geofence-mmdb-recovery \
     --from=cronjob/geofence-mmdb-updater -n <namespace>
   ```

2. **Verify file integrity:**
   ```bash
   # Check file size (should be ~5-10MB)
   kubectl exec -n <namespace> deployment/geofence -c geofence -- \
     ls -lh /data/GeoLite2-Country.mmdb
   ```

---

## Rollback Procedures

### Scenario 1: Bad Database Update

If a database update causes issues:

**Immediate Mitigation:**
Service automatically retains previous working database. No action needed unless pods restart.

**Manual Rollback:**

1. **Restore from PVC snapshot** (if available):
   ```bash
   # List available snapshots
   kubectl get volumesnapshot -n <namespace>
   
   # Restore from a previous snapshot
   kubectl apply -f pvc-rollback.yaml
   ```

2. **Download specific database version** (MaxMind doesn't provide versioned archives for GeoLite2)
   - Contact MaxMind support if reproducible issue
   - Consider maintaining backup of last known-good MMDB

### Scenario 2: Service Degradation After Reload

**Symptoms:**
- Increased latency or error rates after database update
- Application logs show MMDB-related errors

**Rollback Steps:**

1. **Temporarily disable hot-reload:**
   ```bash
   # Scale down to force pod restart with stable database
   kubectl scale deployment geofence --replicas=0 -n <namespace>
   
   # Replace MMDB with backup
   kubectl exec -n <namespace> -c geofence deployment/geofence -- \
     cp /data/GeoLite2-Country.mmdb.bak /data/GeoLite2-Country.mmdb
   
   # Scale back up
   kubectl scale deployment geofence --replicas=3 -n <namespace>
   ```

2. **Pause CronJob temporarily:**
   ```bash
   # Suspend automated updates
   kubectl patch cronjob geofence-mmdb-updater -n <namespace> \
     -p '{"spec":{"suspend":true}}'
   
   # Resume later after investigation
   kubectl patch cronjob geofence-mmdb-updater -n <namespace> \
     -p '{"spec":{"suspend":false}}'
   ```

### Scenario 3: Emergency Service Recovery

Complete service failure requiring immediate recovery:

```bash
# 1. Check if issue is database-related
kubectl logs -n <namespace> -l app=geofence --tail=100

# 2. If MMDB corruption suspected, use test database
kubectl exec -n <namespace> deployment/geofence -c geofence -- \
  wget -O /data/GeoLite2-Country.mmdb \
  https://github.com/maxmind/MaxMind-DB/raw/main/test-data/GeoLite2-Country-Test.mmdb

# 3. Service will auto-reload; verify readiness
kubectl get pods -n <namespace> -l app=geofence

# 4. Restore production database when resolved
kubectl create job geofence-mmdb-restore \
  --from=cronjob/geofence-mmdb-updater -n <namespace>
```

---

## Maintenance Tasks

### Update CronJob Schedule

```bash
# Edit CronJob
kubectl edit cronjob geofence-mmdb-updater -n <namespace>

# Modify schedule field (cron syntax):
# "0 3 * * *"  = Daily at 3 AM UTC
# "0 */12 * * *" = Every 12 hours
# "0 2 * * 0"  = Weekly on Sunday at 2 AM
```

### Rotate MaxMind License Key

```bash
# 1. Generate new license key in MaxMind account
# 2. Update secret in your secrets backend (External Secrets Operator, Sealed Secrets, Vault, etc.)
# 3. Delete existing secret pod to trigger refresh
kubectl delete pods -n <namespace> -l app=geofence-mmdb-updater

# New key takes effect on next CronJob execution
```

### Monitor Storage Usage

```bash
# Check PVC usage
kubectl exec -n <namespace> deployment/geofence -c geofence -- \
  df -h /data

# Database files are small (~5-10MB each)
# Alert if usage exceeds 500MB (indicates accumulation issue)
```

---

## Monitoring & Alerts

### Recommended Alerts

Set up alerts for the following conditions:

1. **CronJob Failure** (Priority: High)
   - Condition: 2 consecutive failed jobs
   - Action: Verify MaxMind credentials and network connectivity

2. **Readiness Probe Failure** (Priority: Critical)
   - Condition: Pod not ready for >5 minutes
   - Action: Check MMDB file integrity and application logs

3. **Stale Database** (Priority: Medium)
   - Condition: MMDB file >7 days old
   - Action: Investigate CronJob execution history

4. **PVC Near Capacity** (Priority: Low)
   - Condition: >80% storage used
   - Action: Clean up old files or expand PVC

### Logging Best Practices

Application logs to monitor (via log aggregation system):

```
# Successful reload
level=INFO msg="Successfully reloaded MMDB database" path=/data/GeoLite2-Country.mmdb

# Failed reload (old DB remains active)
level=ERROR msg="Failed to reload MMDB database, keeping existing reader" error="..."

# File watcher issues
level=WARN msg="File watcher failed to start, hot-reload disabled"
```

---

## Security Considerations

### Secret Management

- **Never commit credentials** to version control
- Use external secret management (Sealed Secrets, External Secrets Operator, Vault)
- Rotate license keys quarterly or after suspected compromise

### Access Control

```bash
# Restrict secret access using RBAC
kubectl create role geofence-secret-reader \
  --verb=get --resource=secrets \
  --resource-name=geofence-secret -n <namespace>
```

### Audit Trail

```bash
# Review who accessed the secret
kubectl get events -n <namespace> --field-selector involvedObject.name=geofence-secret
```

---

## FAQ

**Q: How often should the database be updated?**  
A: MaxMind updates GeoLite2 twice weekly (Tuesdays and Fridays). Daily updates (default schedule) ensure data freshness with minimal overhead.

**Q: What happens if the CronJob fails?**  
A: The service continues using the existing database with zero impact. Hot-reload means no restarts are needed when the job succeeds later.

**Q: Can I disable hot-reload?**  
A: Hot-reload is automatic and cannot be disabled. If file watcher fails to initialize, the service continues without hot-reload (requires manual restarts for updates).

**Q: Does database update cause downtime?**  
A: No. The service uses atomic pointer swaps to reload databases with zero request failures or latency spikes.

**Q: How do I test database updates in staging?**  
A: Manually trigger the CronJob, verify logs show successful reload, and run integration tests against staging environment.

---

## Support & Escalation

### Internal Contacts
- **Service Owner**: Platform Team
- **On-Call**: #platform-oncall Slack channel
- **Database Issues**: Database SRE team

### External Resources
- MaxMind Support: https://support.maxmind.com/
- GeoLite2 Documentation: https://dev.maxmind.com/geoip/geolite2-free-geolocation-data
- geoipupdate Docs: https://github.com/maxmind/geoipupdate

---

**Document Version**: 1.0  
**Last Updated**: February 2026  
**Owner**: Platform Engineering Team
