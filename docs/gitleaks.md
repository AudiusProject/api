# Gitleaks

Scan the repo for secrets, etc.

```
brew install gitleaks

gitleaks detect \
  --source=. \
  --config=.gitleaks.toml \
  --log-opts="--all" \
  --report-format=json \
  --report-path=gitleaks-report.json
```
