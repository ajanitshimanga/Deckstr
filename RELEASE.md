# Release Process

## Standard Release Workflow

**DO NOT manually build or upload artifacts.** The GitHub Actions workflow handles everything.

### Steps to Cut a New Release

1. **Make your changes** and commit them to master:
   ```bash
   git add -A
   git commit -m "feat: description of changes"
   ```

2. **Update version** in `frontend/package.json` (stay in 1.1.x for now):
   ```json
   "version": "1.1.X",
   ```

3. **Push to master**:
   ```bash
   git push origin master
   ```

4. **Create and push a tag**:
   ```bash
   git tag v1.1.X
   git push origin v1.1.X
   ```

5. **Wait for GitHub Actions** - The workflow will:
   - Run tests (must pass)
   - Build the Windows application
   - Create Inno Setup installer
   - Create/update the GitHub release with the installer

### Version Numbering

- **1.1.X** - Current minor version series (bug fixes, small features)
- **1.2.0** - Next minor version (when ready for larger feature set)
- **2.0.0** - Major version (breaking changes)

### Monitoring the Release

```bash
# Watch the workflow progress
gh run watch

# Check release status
gh release view v1.1.X
```

### If Something Goes Wrong

1. **Failed tests**: Fix the code, amend or create new commit, force push tag:
   ```bash
   git tag -d v1.1.X
   git push origin :refs/tags/v1.1.X
   # After fixes...
   git tag v1.1.X
   git push origin v1.1.X
   ```

2. **Delete a bad release**:
   ```bash
   gh release delete v1.1.X --yes
   git push origin :refs/tags/v1.1.X
   ```

### What NOT to Do

- Do NOT run `wails build` locally and upload artifacts
- Do NOT create releases manually with `gh release create`
- Do NOT skip the tag - the workflow triggers on `v*` tags only
