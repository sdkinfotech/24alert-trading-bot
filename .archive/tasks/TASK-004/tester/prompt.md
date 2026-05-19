# Промпт: Тестировщик → TASK-004

## Контекст

Ты — **Тестировщик**. DevOps создал GitHub Actions workflow и deployment скрипты. Теперь нужно валидировать весь end-to-end процесс: от push до production deployment.

**Исходные данные**:
- `TASK-004/task.md` — requirements
- `TASK-004/devops/handoff.md` — что создал DevOps
- `.github/workflows/deploy.yml` — workflow file
- `scripts/deploy-prod.sh`, `scripts/rollback-prod.sh` — scripts

---

## Задача

### 1. E2E Test: Push → Production Deployment

**Test Case 1: Full Workflow Execution**
1. Make a dummy code change (e.g., add comment in README)
2. Commit: `git commit -am "test: dummy commit for CI/CD validation"`
3. Push: `git push origin main`
4. Monitor GitHub Actions UI — check workflow runs
5. Verify:
   - ✅ Test job passes (all tests, coverage >= 70%)
   - ✅ Build job passes (Docker images tagged & pushed)
   - ✅ Deploy job passes (SSH to srv03-cloud, containers up)
   - ✅ Notify job sends Slack message
6. Verify production state:
   - ✅ SSH to srv03-cloud: `docker-compose ps` (all healthy)
   - ✅ Call API: `curl http://176.123.160.234:8080/health` (200 OK)
   - ✅ Check latest commit: `git log --oneline -1` matches pushed commit

**Expected Duration**: 10 minutes total

---

### 2. Smoke Tests (Post-Deployment)

After each deployment, verify:

**Test 2: Health Check Endpoints**
```bash
curl -v http://176.123.160.234:8080/health
# Expected: 200 OK, JSON response
```

**Test 3: API Endpoints**
```bash
curl http://176.123.160.234:8080/api/accounts
curl http://176.123.160.234:8080/api/portfolio
curl http://176.123.160.234:8080/api/risk-status
# Expected: 200 OK, valid JSON
```

**Test 4: Swagger UI**
```
http://176.123.160.234:8080/swagger/
# Expected: 200 OK, interactive docs visible
```

**Test 5: Logs**
```bash
ssh ubuntu@176.123.160.234 "docker-compose logs --tail=50"
# Expected: No ERROR or PANIC entries
```

---

### 3. Rollback Test (Critical)

**Test 6: Simulate Deployment Failure & Rollback**

1. Record current deployed version:
   ```bash
   docker pull <image>:<tag> && docker image inspect <image>:<tag> --format='{{.RepoDigests}}'
   ```

2. Manually corrupt a container to trigger health check failure:
   ```bash
   ssh ubuntu@176.123.160.234 "docker exec gateway-service bash -c 'kill 1'"
   ```

3. Trigger rollback script:
   ```bash
   bash scripts/rollback-prod.sh 176.123.160.234 ubuntu <previous-image>
   ```

4. Verify recovery:
   - ✅ Containers restarted
   - ✅ Health check passes
   - ✅ API responds normally

---

### 4. Slack Notification Test

**Test 7: Slack Notifications**

1. After a successful deployment, check Slack channel (`#deployments` or `#alerts`)
2. Verify message contains:
   - ✅ Status: ✅ or ❌
   - ✅ Commit SHA (first 7 chars)
   - ✅ Timestamp
   - ✅ Link to GitHub Actions run
   - ✅ Author/pusher name (if available)

**Test 8: Slack on Failure Notification**

1. Modify `.github/workflows/deploy.yml` locally to fail a test (e.g., set coverage to 100%)
2. Push branch & verify Slack gets ❌ notification
3. Revert change

---

### 5. Log & Security Tests

**Test 9: No Secrets in Logs**

1. Review GitHub Actions logs for any leaked secrets:
   ```bash
   # Check for token/key patterns in logs
   grep -i "token\|key\|secret\|password" /path/to/logs
   # Expected: NO matches
   ```

2. Check SSH key is not printed:
   ```bash
   # In GitHub Actions output, search for:
   # - "BEGIN RSA PRIVATE KEY"
   # - "BEGIN OPENSSH PRIVATE KEY"
   # Expected: NO matches
   ```

---

### 6. Performance Tests

**Test 10: Deployment Duration**

1. Record workflow execution time in GitHub Actions UI
2. Verify:
   - ✅ Total time < 10 minutes
   - ✅ Deploy job < 2 minutes
   - ✅ Health check < 30 seconds

---

### 7. Idempotency Test

**Test 11: Deploy Twice in a Row**

1. Push same commit twice (force-push, then revert, then push again)
2. Verify:
   - ✅ Second deployment succeeds
   - ✅ System state identical after both deploys
   - ✅ No duplicate data or containers

---

## Test Execution Plan

| # | Test | Command | Expected Result | Duration |
|---|------|---------|---|---|
| 1 | Full workflow | `git push origin main` | All jobs pass | 10 min |
| 2 | Health check | `curl /health` | 200 OK | < 1 min |
| 3 | API endpoints | `curl /api/...` | 200 OK | < 1 min |
| 4 | Swagger UI | `curl /swagger/` | 200 OK | < 1 min |
| 5 | Logs clean | `docker-compose logs` | No errors | < 1 min |
| 6 | Rollback | Manual failure + script | Recovered | 5 min |
| 7 | Slack notify | Check channel | Message received | < 1 min |
| 8 | Slack failure | Trigger failure | ❌ message | 10 min |
| 9 | No secrets leaked | Grep logs | No matches | < 1 min |
| 10 | Duration < 10 min | GitHub UI | Total < 10 min | — |
| 11 | Idempotency | Push twice | Identical state | 20 min |

**Total Time**: ~2 hours

---

## What to Include in handoff.md

```markdown
# Handoff: Tester → TASK-004

## Статус
DONE

## Что протестировано
- ✅ E2E workflow: push → deploy → production (11/11 tests passed)
- ✅ Smoke tests: health check, API endpoints, Swagger UI
- ✅ Rollback: manual failure simulation → recovery
- ✅ Slack notifications: success & failure messages
- ✅ Security: no secrets in logs
- ✅ Performance: < 10 min total, < 2 min deploy
- ✅ Idempotency: deploy twice = same state

## Артефакты
- Файл: test-results.md (full log of all 11 tests)
- Link: GitHub Actions run #1 (successful deployment log)

## Корректировки для следующих ролей
НЕ ТРЕБУЕТСЯ
[или если найдены проблемы]

## Блокеры
НЕТ
```

---

## Success Criteria

✅ All 11 tests pass  
✅ E2E workflow executes without manual intervention  
✅ Rollback works reliably  
✅ Slack notifications received  
✅ No secrets leaked  
✅ Ready for Tech Lead review
