# Deployment Guide — Trading Bot CI/CD

## Overview

This document describes the GitHub Actions CI/CD pipeline for the 24Alert Trading Bot. The pipeline automatically tests, builds, and deploys code to production on every push to `main` branch.

### Pipeline Flow

```
git push origin main
    ↓
[Test] Run linting, tests, build locally (Ubuntu, Go 1.25)
    ↓ (on success)
[Build] Docker build & push images to Docker Hub
    ↓ (on success)
[Deploy] SSH to srv03-cloud → git pull → docker-compose up → health check
    ↓ (on success)
[Notify] Send Slack message (status, commit, logs link)
```

**Total Pipeline Duration**: ~10 minutes (depends on Docker build size)

---

## GitHub Secrets Configuration

The pipeline requires 5 secrets to be configured in GitHub repository settings:

### 1. `DOCKER_USERNAME`
**Value**: Your Docker Hub username  
**Used in**: Build job (docker login)  
**Setup**:
- Go to: https://hub.docker.com/settings/security
- Create or use existing Docker Hub account
- GitHub repo → Settings → Secrets and variables → Actions → New repository secret
  - Name: `DOCKER_USERNAME`
  - Value: (your Docker Hub username)

### 2. `DOCKER_TOKEN`
**Value**: Docker Hub Personal Access Token  
**Used in**: Build job (docker push)  
**Setup**:
- Go to: https://hub.docker.com/settings/security → New Access Token
- Name it: `github-actions`
- Permissions: `Read & Write`
- Copy token
- GitHub repo → Settings → Secrets and variables → Actions → New repository secret
  - Name: `DOCKER_TOKEN`
  - Value: (paste token)

### 3. `DEPLOY_HOST`
**Value**: IP address or hostname of production server  
**Used in**: Deploy job (SSH target)  
**Setup**:
- GitHub repo → Settings → Secrets and variables → Actions → New repository secret
  - Name: `DEPLOY_HOST`
  - Value: `176.123.160.234`

### 4. `DEPLOY_USER`
**Value**: SSH username on production server  
**Used in**: Deploy job (SSH authentication)  
**Setup**:
- GitHub repo → Settings → Secrets and variables → Actions → New repository secret
  - Name: `DEPLOY_USER`
  - Value: `adm-srv03-cloud` (or whatever user has permissions on /opt/24alert)

### 5. `DEPLOY_KEY`
**Value**: Private SSH key (PEM format)  
**Used in**: Deploy job (SSH private key for authentication)  
**Setup**:

#### Generate SSH key (if not already created):
```bash
# On local machine
ssh-keygen -t ed25519 -C "github-actions-deploy" -f ~/.ssh/github_deploy_key

# Or use existing key
cat ~/.ssh/id_ed25519  # if you have one
```

#### Add public key to server:
```bash
# Copy public key to server authorized_keys
cat ~/.ssh/github_deploy_key.pub | ssh adm-srv03-cloud@176.123.160.234 "cat >> ~/.ssh/authorized_keys"

# Or manually:
scp ~/.ssh/github_deploy_key.pub adm-srv03-cloud@176.123.160.234:~/
ssh adm-srv03-cloud@176.123.160.234 "cat ~/github_deploy_key.pub >> ~/.ssh/authorized_keys && chmod 600 ~/.ssh/authorized_keys"
```

#### Add private key to GitHub:
```bash
# Copy private key
cat ~/.ssh/github_deploy_key

# GitHub repo → Settings → Secrets and variables → Actions → New repository secret
# Name: DEPLOY_KEY
# Value: (paste entire private key including -----BEGIN OPENSSH PRIVATE KEY----- lines)
```

### 6. `SLACK_WEBHOOK` (Optional)
**Value**: Slack incoming webhook URL  
**Used in**: Notify job (send deployment status messages)  
**Setup**:
- Go to: https://api.slack.com/apps
- Create New App → From scratch
  - App name: `Trading Bot Deployments`
  - Workspace: (select your workspace)
- Left menu → Incoming Webhooks → On
- Add New Webhook to Workspace
- Select channel: `#deployments` (or any channel)
- Copy Webhook URL
- GitHub repo → Settings → Secrets and variables → Actions → New repository secret
  - Name: `SLACK_WEBHOOK`
  - Value: (paste webhook URL)

---

## Verifying Secrets Are Set

```bash
# Check which secrets are configured (doesn't show values)
gh secret list --repo sdkinfotech/24alert-trading-bot
```

---

## Health Check Endpoints

The deployment validates readiness by checking these endpoints:

### Gateway Health (Primary)
```bash
curl -f http://localhost:8080/health

# Expected response:
# HTTP/1.1 200 OK
# {
#   "status": "ok",
#   "timestamp": "2026-04-03T10:00:00Z"
# }
```

### Individual Service Health (for debugging)
```bash
# Order Service
curl -f http://localhost:9001/health

# Market Data Service
curl -f http://localhost:9002/health

# Portfolio Service
curl -f http://localhost:9003/health

# Risk Service
curl -f http://localhost:9004/health
```

---

## Manual Deployment (without GitHub Actions)

If you need to deploy manually without pushing to GitHub:

### Option 1: Using deploy script directly
```bash
# On local machine with SSH key and access to srv03-cloud
bash scripts/deploy-prod.sh 176.123.160.234 adm-srv03-cloud $(git rev-parse --short HEAD)
```

### Option 2: Manual SSH deployment
```bash
# SSH to server
ssh adm-srv03-cloud@176.123.160.234

# On server:
cd /opt/24alert

# Pull latest code
git pull origin main

# Build and restart
docker-compose -f deployments/docker-compose.yaml build
docker-compose -f deployments/docker-compose.yaml up -d

# Verify
curl http://localhost:8080/health
docker-compose ps
```

---

## Rollback Procedure

### Automatic Rollback (if health check fails)
The pipeline will automatically attempt to use the previous image if deployment fails.

### Manual Rollback

#### Using rollback script:
```bash
# Rollback to previous tagged image
bash scripts/rollback-prod.sh 176.123.160.234 adm-srv03-cloud sdkinfotech/24alert-trading-bot:main-abc1234
```

#### Manual rollback steps:
```bash
# SSH to server
ssh adm-srv03-cloud@176.123.160.234

# On server:
cd /opt/24alert

# Stop current containers
docker-compose down

# Pull previous image
docker pull sdkinfotech/24alert-trading-bot:main-<commit-hash>

# Or use latest stable
docker pull sdkinfotech/24alert-trading-bot:latest

# Restart
docker-compose up -d

# Verify
sleep 3
curl http://localhost:8080/health
docker-compose ps
```

---

## SSH Key Rotation

### Recommended: Rotate every 90 days

#### Step 1: Generate new key
```bash
ssh-keygen -t ed25519 -C "github-actions-deploy-$(date +%Y%m%d)" -f ~/.ssh/github_deploy_key_new
```

#### Step 2: Add new public key to server
```bash
cat ~/.ssh/github_deploy_key_new.pub | ssh adm-srv03-cloud@176.123.160.234 "cat >> ~/.ssh/authorized_keys"
```

#### Step 3: Update GitHub secret
```bash
# Copy new private key and update DEPLOY_KEY secret in GitHub
cat ~/.ssh/github_deploy_key_new

# GitHub → Settings → Secrets → DEPLOY_KEY → Update
```

#### Step 4: Verify with test deployment
```bash
# Push a small change to main to trigger CI/CD
git commit --allow-empty -m "test: verify new deploy key"
git push origin main

# Monitor: GitHub → Actions → watch workflow
```

#### Step 5: Remove old key from server
```bash
# After verification, remove old key from authorized_keys
ssh adm-srv03-cloud@176.123.160.234

# On server, edit ~/.ssh/authorized_keys and remove the old line
# Or use:
grep -v "github-actions-deploy-old" ~/.ssh/authorized_keys > ~/.ssh/authorized_keys.tmp
mv ~/.ssh/authorized_keys.tmp ~/.ssh/authorized_keys
chmod 600 ~/.ssh/authorized_keys
```

#### Step 6: Delete old local key
```bash
rm ~/.ssh/github_deploy_key
```

---

## Monitoring & Logs

### View workflow logs
```bash
# Via GitHub CLI
gh run list --repo sdkinfotech/24alert-trading-bot
gh run view <run-id> --log

# Via GitHub web
# https://github.com/sdkinfotech/24alert-trading-bot/actions
```

### Common workflow statuses
- ✅ **Success**: All jobs completed successfully
- ❌ **Failure**: Test, build, or deploy job failed (see log details)
- ⏸️ **Cancelled**: Workflow was manually stopped
- ⚠️ **Skipped**: Deploy job skipped (e.g., not on main branch)

### Real-time monitoring
```bash
# Watch workflow status
gh run watch <run-id> --repo sdkinfotech/24alert-trading-bot
```

---

## Troubleshooting

### Issue: Workflow fails at `Test` step

**Symptom**: `make ci-check` fails

**Causes**:
- Go syntax error
- Linting issues (golangci-lint)
- Test failures

**Fix**:
```bash
# Run locally to see error
make ci-check

# Fix issues and commit
git add .
git commit -m "fix: linting errors"
git push origin main
```

### Issue: Workflow fails at `Build` step

**Symptom**: Docker image build fails

**Cause**: Usually Dockerfile or dependency issue

**Fix**:
```bash
# Build locally
docker-compose -f deployments/docker-compose.yaml build

# Check for errors in Dockerfile or dependencies
# Fix and push
```

### Issue: Workflow fails at `Deploy` step

**Symptom**: SSH connection fails or health check fails

**Causes**:
- SSH key not configured correctly
- Server unreachable
- Application failed to start

**Debug**:
```bash
# Test SSH access
ssh -i ~/.ssh/deploy_key adm-srv03-cloud@176.123.160.234 "echo OK"

# Test application status
ssh adm-srv03-cloud@176.123.160.234
docker-compose -f deployments/docker-compose.yaml ps
docker-compose logs --tail=50
```

### Issue: Docker Hub authentication fails

**Symptom**: `denied: requested access to the resource is denied` during push

**Cause**: Invalid Docker token or username

**Fix**:
```bash
# Test locally
docker login -u <username> -p <token>
docker push sdkinfotech/24alert-trading-bot:test

# Update GitHub secrets if needed
# Settings → Secrets → DOCKER_USERNAME / DOCKER_TOKEN
```

### Issue: Health check times out

**Symptom**: Deploy step hangs for 5 minutes

**Cause**: Application slow to start or crashed

**Fix**:
```bash
# SSH to server and check manually
ssh adm-srv03-cloud@176.123.160.234
curl http://localhost:8080/health
docker-compose logs gateway

# Check resource constraints
docker stats
```

### Issue: Slack notification fails

**Symptom**: No Slack message appears

**Cause**: Invalid webhook URL or Slack workspace permissions

**Fix**:
```bash
# Test webhook manually
curl -X POST -H 'Content-type: application/json' \
  --data '{"text":"Test message"}' \
  https://hooks.slack.com/services/T.../B.../...

# Update SLACK_WEBHOOK secret if needed
# Ensure bot has permission to post in channel
```

---

## On-Call Runbook

### Deployment Issues

1. **Check workflow status**
   ```bash
   gh run list --repo sdkinfotech/24alert-trading-bot
   ```

2. **View logs**
   ```bash
   gh run view <run-id> --log
   ```

3. **If deploy failed**
   - Look at error message in logs
   - Check production server status: `ssh adm-srv03-cloud@176.123.160.234`
   - Verify health: `curl http://176.123.160.234:8080/health`

4. **If rollback needed**
   ```bash
   bash scripts/rollback-prod.sh 176.123.160.234 adm-srv03-cloud sdkinfotech/24alert-trading-bot:latest
   ```

5. **If SSH access fails**
   - Verify `DEPLOY_KEY` secret is set correctly
   - Test key locally: `ssh -i ~/.ssh/deploy_key adm-srv03-cloud@176.123.160.234`
   - Verify server IP in `DEPLOY_HOST` secret

6. **If service won't start**
   - Check logs: `docker-compose logs`
   - Verify .env configuration with production tokens
   - Check disk space: `df -h`
   - Check Docker status: `docker ps`

### Incident Response

- **Minor issue (health check retried)**: Monitor logs, no action needed
- **Deploy failed**: Check logs, fix code, push again
- **Server down**: SSH into srv03-cloud, check Docker status, manually restart if needed
- **Network partition**: Wait for resolution, health checks will retry automatically
- **Credentials expired**: Rotate DEPLOY_KEY or DOCKER_TOKEN secrets

---

## Best Practices

1. **Always test locally before pushing**
   ```bash
   make ci-check
   docker-compose build
   ```

2. **Use meaningful commit messages**
   ```bash
   git commit -m "feat: add new API endpoint for accounts"
   ```

3. **Monitor first deployment after changes**
   - Watch GitHub Actions logs in real-time
   - Verify production health after deploy

4. **Keep SSH key secure**
   - Don't share in email or chat
   - Use separate key for GitHub Actions
   - Rotate regularly

5. **Document any manual changes to production**
   - Log in GitHub issues or PR comments
   - Update runbook if procedure changes

---

## File Locations

| File | Location | Purpose |
|------|----------|---------|
| Workflow | `.github/workflows/deploy.yml` | GitHub Actions pipeline definition |
| Deploy script | `scripts/deploy-prod.sh` | SSH deployment automation |
| Rollback script | `scripts/rollback-prod.sh` | Manual rollback automation |
| Makefile | `Makefile` | Local test/build targets |
| Docker Compose | `deployments/docker-compose.yaml` | Service definitions |

---

## Additional Resources

- [GitHub Actions Documentation](https://docs.github.com/en/actions)
- [Docker Hub API](https://docs.docker.com/docker-hub/api/latest/)
- [SSH Key Management](https://help.github.com/en/articles/generating-a-new-ssh-key-and-adding-it-to-the-ssh-agent)
- [Slack Webhook API](https://api.slack.com/messaging/webhooks)

---

**Last updated**: 2026-04-03  
**Maintained by**: DevOps Team  
**Questions?**: Check workflow logs or GitHub Actions documentation
