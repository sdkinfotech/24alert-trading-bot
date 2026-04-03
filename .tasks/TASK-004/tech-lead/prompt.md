# Промпт: Tech-Lead → TASK-004

## Контекст

Ты — **Tech-Lead**. Tester валидировал весь workflow и все тесты прошли. Теперь нужна финальная review:
- Security
- Best practices
- Production readiness

**Исходные данные**:
- `TASK-004/tester/handoff.md` — результаты тестирования (все 11 тестов passed)
- `.github/workflows/deploy.yml` — GitHub Actions workflow
- `scripts/deploy-prod.sh`, `scripts/rollback-prod.sh` — deployment scripts
- `.github/DEPLOYMENT.md` — documentation

---

## Задача

### 1. Security Review

**1.1 GitHub Secrets Management**
- ✅ Check: All sensitive data in Secrets, not in YAML
  - Look for: DOCKER_TOKEN, DEPLOY_KEY, SLACK_WEBHOOK
  - Expected: Should be referenced as `${{ secrets.DEPLOY_KEY }}`, not hardcoded
- ✅ Check: SSH key format & permissions
  - Deploy key should be: `-----BEGIN OPENSSH PRIVATE KEY-----`
  - Permissions: 600 (read-only by owner)
- ✅ Check: Key rotation policy documented
  - Recommendation: Rotate every 90 days, docs should explain

**1.2 Network Security**
- ✅ Check: SSH deployment uses key-based auth (no passwords)
- ✅ Check: Health checks don't expose sensitive endpoints
- ✅ Check: No API tokens/credentials in deploy logs

**1.3 Code Injection Risks**
- ✅ Check: No user input in SSH commands (scripts use fixed vars)
- ✅ Check: Docker images pulled from trusted registry only
- ✅ Check: Rollback can't be triggered by untrusted commits

---

### 2. Best Practices Review

**2.1 Idempotency**
- ✅ Verify: Deploy script is idempotent (can run twice safely)
  - Test: `docker-compose up` is idempotent (only replaces changed images)
  - Verify: No state accumulation (e.g., logs, data) on repeated runs
  - Recommendation: Add `--remove-orphans` flag to `docker-compose up`

**2.2 Error Handling & Timeouts**
- ✅ Check: `set -e` in all bash scripts (exit on first error)
- ✅ Check: Health check has retry logic (not single attempt)
- ✅ Check: Deploy timeout < 5 minutes total (documented)
- ✅ Check: Rollback timeout < 2 minutes

**2.3 Logging & Observability**
- ✅ Check: All steps log meaningful messages ([DEPLOY], [ROLLBACK], etc.)
- ✅ Check: Errors are descriptive (not generic "failed")
- ✅ Check: Slack notifications include links to GitHub Actions run
- ✅ Recommendation: Add correlation IDs for audit trail

**2.4 Deployment Strategy**
- ✅ Verify: Graceful shutdown before restart
  - Check: `docker-compose` has `stop_grace_period` set
  - Recommendation: 30-60 seconds for graceful drain
- ✅ Verify: Health checks cover all critical services
  - All 5 services should respond to `/health` endpoint
  - Recommendation: Add service-specific health endpoints

**2.5 Documentation**
- ✅ Check: `.github/DEPLOYMENT.md` covers:
  - How to trigger manual deployment
  - GitHub Secrets setup
  - SSH key rotation
  - Health check endpoints
  - Rollback procedure (manual)
  - Troubleshooting guide
  - On-call runbook

---

### 3. Production Readiness

**3.1 Workflow Coverage**
- ✅ Does workflow run on all merges to `main`? YES
- ✅ Does workflow prevent merging of failing tests? (Recommend: GitHub branch protection rule)
- ✅ Are matrix tests sufficient (Go versions, OS)? Recommend: ubuntu-latest only for now
- ✅ Are all docker services tested post-deploy? YES (health check)

**3.2 Failure Scenarios**
- ✅ Test fails → Deploy skipped ✅
- ✅ Docker build fails → Deploy skipped ✅
- ✅ SSH auth fails → Rollback triggered ✅
- ✅ Health check fails → Rollback triggered ✅
- ✅ Slack webhook fails → Deploy doesn't rollback (warning only) ✅

**3.3 Monitoring & On-Call**
- ✅ Is on-call runbook included in `.github/DEPLOYMENT.md`? YES/NO
  - Should include: common issues, debug steps, escalation
- ✅ Are deployment logs retained for audit? GitHub Actions retention policy?
  - Recommendation: Set to 90 days minimum

---

### 4. Code Quality

**4.1 YAML Syntax**
- ✅ Validate: `.github/workflows/deploy.yml` is valid YAML
  - Use: `yamllint` or GitHub's built-in validator
  - Check: Indentation (2 spaces), keys quoted where needed

**4.2 Bash Scripts**
- ✅ Check: `shellcheck` passes on `scripts/deploy-prod.sh` and `scripts/rollback-prod.sh`
  - Look for: undefined variables, unquoted strings, etc.
- ✅ Check: Scripts have meaningful comments
- ✅ Check: Scripts are executable (`chmod +x`)

**4.3 Documentation**
- ✅ Check: All files have clear READMEs / inline comments
- ✅ Check: No typos or outdated info

---

## Review Checklist

### Security
- [ ] All secrets in GitHub Secrets (not hardcoded)
- [ ] SSH key is private (600 permissions)
- [ ] Key rotation policy documented
- [ ] Health checks don't expose sensitive data
- [ ] No API tokens in logs

### Best Practices
- [ ] Deploy script is idempotent
- [ ] Error handling: `set -e`, meaningful errors
- [ ] Timeouts documented & reasonable (< 5 min total)
- [ ] Health check has retry logic
- [ ] Graceful shutdown (stop_grace_period)
- [ ] Logging is meaningful & audit-friendly

### Production Readiness
- [ ] Workflow runs on all `main` pushes
- [ ] Test failures prevent deploy
- [ ] Rollback tested & working
- [ ] Slack notifications reliable
- [ ] On-call runbook complete
- [ ] Deployment logs retained 90+ days

### Code Quality
- [ ] YAML valid & well-formatted
- [ ] Bash scripts pass `shellcheck`
- [ ] Scripts are executable
- [ ] Documentation clear & complete

---

## Sign-Off Decision

After review, make a final decision:

**IF all items pass** → APPROVED
```markdown
# Handoff: Tech-Lead → TASK-004

## Статус
APPROVED

## Проверено
- ✅ Security: Secrets, SSH key, auth
- ✅ Best practices: Idempotency, timeouts, error handling
- ✅ Production readiness: Workflow coverage, failure scenarios
- ✅ Code quality: YAML, bash scripts

## Что одобрено
- ✅ GitHub Actions workflow
- ✅ Deployment scripts
- ✅ Rollback procedure
- ✅ Documentation

## Условия для следующего этапа
НЕ ТРЕБУЕТСЯ

## Рекомендации для TASK-005
- Consider Kubernetes for advanced rollout strategies (Canary, Blue-Green)
- Add Prometheus metrics for deployment success rate
- Integrate with incident management on deploy failure
```

**IF issues found** → NEEDS_CORRECTION
```markdown
# Handoff: Tech-Lead → TASK-004

## Статус
NEEDS_CORRECTION

## Найденные проблемы
1. SSH key hardcoded in workflow YAML (Security risk)
   - Action: Move to GitHub Secrets
2. Health check has no retry logic (fragile)
   - Action: Add 3x retry with 2s delay
3. Documentation missing troubleshooting guide
   - Action: Add common issues & fixes

## Следующие шаги
- DevOps: Fix issues, update workflow & scripts
- Tester: Re-validate after corrections
- Tech-Lead: Final sign-off
```

---

## Success Criteria

✅ Security review passed (no risk findings)  
✅ Best practices review passed (idempotent, reliable, documented)  
✅ Production readiness confirmed  
✅ Code quality verified  
✅ Decision: APPROVED (or issues documented for correction)
