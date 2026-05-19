# TASK-004: CI/CD Pipeline (GitHub Actions)

**Priority**: HIGH  
**Complexity**: M  
**Effort Estimate**: 3–5 дней  
**Phase**: 2 (Stabilization)  
**Depends on**: TASK-003 (Production Deployment Complete)

---

## Goal

Полностью автоматизированный CI/CD пайплайн для 24alert trading bot. При каждом `git push origin main`:
1. ✅ Все тесты проходят (unit + integration)
2. ✅ Code quality checks (linting, code coverage)
3. ✅ Docker образы собираются
4. ✅ Образы заливаются в registry (Docker Hub / GitHub Packages)
5. ✅ Автоматический deploy на production (srv03-cloud)
6. ✅ Health checks валидируют готовность системы
7. ✅ На ошибку — rollback к предыдущей версии
8. ✅ Notификации в Slack (status, link to logs)

**Result**: Zero-click deployment на production. Developer пушит код → автоматически на сервер.

---

## Scope

### In Scope

- **GitHub Actions Workflow** (`.github/workflows/deploy.yml`)
  - Trigger: `push main` branch
  - Matrix: Go 1.21+
  - Cache: Go modules, Docker layers

- **Testing Pipeline**
  - `go test -v -cover ./...` (all packages)
  - `golangci-lint` (linting)
  - Code coverage threshold: 70% minimum
  - Stop on test failures

- **Docker Build & Push**
  - Multi-stage builds (production size optimization)
  - Push to Docker Hub / GitHub Container Registry
  - Tag strategy: `latest`, `v1.0.0-sha`, `main`

- **Deployment to Production**
  - SSH key authentication (via GitHub Secrets)
  - Deploy strategy: `git pull origin main` → `docker-compose up`
  - Health check: `curl http://localhost:8080/health` (200 OK)
  - Timeout: 5 minutes max
  - Rollback on failure: revert to previous image, `docker-compose up`

- **Notifications**
  - Slack webhook integration
  - Message: status (✅/❌), commit hash, timestamp, logs link
  - On success & on failure

- **Secrets Management**
  - GitHub Secrets for: `DEPLOY_KEY`, `DEPLOY_HOST`, `DEPLOY_USER`, `DOCKER_TOKEN`, `SLACK_WEBHOOK`
  - No hardcoded credentials in repo
  - Key rotation docs

- **Configuration**
  - `Makefile` targets: `ci-test`, `ci-build`, `ci-deploy` (local CI simulation)
  - Scripts in `scripts/ci-*.sh` (reusable)
  - GitHub Actions YAML validation

### Out of Scope

- Kubernetes deployment (→ TASK-005)
- Multi-environment (dev/staging/prod) branching strategy (v2)
- Advanced rollback strategies (Canary, Blue-Green) (→ TASK-005)
- Private Docker registry setup (use GitHub Packages for now)
- GitHub Actions cost optimization (not critical for MVP)

---

## Requirements

### Functional

| ID | Requirement | Verification |
|----|----|---|
| FR-1 | Tests run automatically on every push | Check GitHub Actions log shows `go test` output |
| FR-2 | Code coverage tracked | Coverage report shown in logs |
| FR-3 | Linting enforced | Workflow fails if `golangci-lint` finds errors |
| FR-4 | Docker images built & pushed | Image appears in Docker Hub with correct tags |
| FR-5 | Deploy happens automatically | SSH into srv03-cloud, check container version |
| FR-6 | Health check validates | Request to `/health` returns 200 OK |
| FR-7 | Rollback on failure | Simulate deploy failure, check previous version restored |
| FR-8 | Slack notifications sent | Message appears in Slack channel |

### Non-Functional

| ID | Requirement | Target |
|----|---|---|
| NFR-1 | Pipeline duration | < 10 minutes (test + build + deploy) |
| NFR-2 | Deployment latency | < 2 minutes (SSH + pull + restart) |
| NFR-3 | Availability during deploy | Graceful restart (connections drained) |
| NFR-4 | Secrets never logged | Grep logs for token patterns (fail if found) |
| NFR-5 | Idempotency | Deploy twice in a row = same state |

---

## Definition of Done (DoD)

- ✅ All tests pass (100% of test suite)
- ✅ Code coverage >= 70%
- ✅ `golangci-lint` runs cleanly (no warnings → errors)
- ✅ Docker images build locally: `docker-compose build`
- ✅ Images pushed to registry with correct tags
- ✅ GitHub Actions workflow executes end-to-end without manual steps
- ✅ Deployment to srv03-cloud succeeds without SSH intervention
- ✅ Health check passes (all 5 services respond)
- ✅ Rollback tested and working (simulated failure, recovery)
- ✅ Slack notifications sent for success & failure
- ✅ No secrets or tokens visible in logs
- ✅ E2E deploy test passes (push → production updated)
- ✅ Documentation: `.github/DEPLOYMENT.md` with troubleshooting
- ✅ Tech Lead approved (security, best practices)

---

## Timeline

| Phase | Owner | Duration | Deliverable |
|-------|-------|----------|---|
| **Backend** | Backend | 1 day | `go test`, `golangci-lint` config, `Makefile` targets |
| **DevOps** | DevOps | 2 days | GitHub Actions workflow, deploy scripts, secrets setup |
| **Tester** | Tester | 1 day | E2E tests, rollback validation, Slack verification |
| **Tech-lead** | Tech-lead | 0.5 day | Security review, production readiness sign-off |
| **Buffer** | — | 0.5 day | Fixes, re-tests |
| **TOTAL** | — | **5 days** | — |

---

## Risks & Mitigation

| Risk | Impact | Mitigation |
|---|---|---|
| GitHub Actions quota exceeded | Workflow throttled | Monitor usage, use self-hosted runner if needed |
| SSH timeout to srv03-cloud | Deploy fails silently | Timeout: 5 min, retry logic, Slack alert |
| Deploy key compromised | Security breach | Rotate key weekly, audit GitHub Actions logs |
| Health check false positive | Rollback on bad data | Multiple health check endpoints, retry logic |
| Previous version unreachable | Rollback fails | Keep 2 stable images, manual intervention docs |
| Slack webhook expires | Notifications silent | Test webhook monthly, alert on 404 |
| Network partition during deploy | Inconsistent state | Idempotent scripts, state validation |

---

## Dependencies

- **TASK-003**: Production deployment running (prerequisite)
- **GitHub repository**: Already set up (tinvest-api-bot)
- **Secrets infrastructure**: GitHub account access
- **Docker Hub**: Registry account (push permissions)

---

## Notes

- **Integration with existing tools**: Build on `Makefile` (already has test, docker targets)
- **Local CI simulation**: Developers can run `make ci-test` locally before pushing
- **Documentation**: Include `.github/workflows/README.md` explaining each step
- **Monitoring**: Log all deploy actions for audit trail
- **Slack channel**: Use `#deployments` channel for notifications (or `#alerts` if preferred)

---

## Related Links

- Repo: `git@github.com:sdkinfotech/tinvest-api-bot.git`
- GitHub Actions Docs: https://docs.github.com/en/actions
- Docker Hub: https://hub.docker.com/
- Slack Webhooks: https://api.slack.com/messaging/webhooks
- Previous deployment: `.tasks/TASK-003/devops/DEPLOYMENT.md`
