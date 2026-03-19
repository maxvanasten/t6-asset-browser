# Release Process Guide

This document contains step-by-step instructions for releasing a new version of t6-asset-browser.

**Last Updated:** 2026-03-18  
**Current Version:** v0.6.0

## Prerequisites

Before starting, ensure you have:

- [ ] Git repository cloned locally
- [ ] Go 1.21+ installed (`go version` to check)
- [ ] Git configured with your credentials
- [ ] Write access to https://github.com/maxvanasten/t6-asset-browser
- [ ] All tests passing (`go test ./...`)
- [ ] OpenAssetTools installed (for testing the built binaries)

## Quick Reference

```bash
# Full release process in one go:
git checkout main && git pull origin main
# Fix version in code if needed
git tag -a v0.1.0 -m "Initial release"
git push origin v0.1.0
./scripts/build-releases.sh
# Then create GitHub release manually and upload files from dist/
```

---

## Detailed Step-by-Step Process

### Step 1: Prepare for Release (5 minutes)

1. **Ensure you're on main branch:**
   ```bash
   git checkout main
   git pull origin main
   ```

2. **Verify working tree is clean:**
   ```bash
   git status
   ```
   
   Should output: `nothing to commit, working tree clean`
   
   If not clean:
   ```bash
   git add .
   git commit -m "Pre-release cleanup"
   git push origin main
   ```

3. **Run all tests:**
   ```bash
   go test ./...
   ```
   
   All tests must pass. If any fail, fix before proceeding.

4. **Decide version number:**
   
   Follow [Semantic Versioning](https://semver.org/):
   - `v0.1.0` - Initial release
   - `v0.1.1` - Bug fixes only
   - `v0.2.0` - New features added
   - `v1.0.0` - Stable, production-ready
   
   For this project, while in development, stay in 0.x.x range.

### Step 2: Update Version References (if needed)

Check if version is hardcoded anywhere:

```bash
grep -r "0.1.0" --include="*.go" --include="*.md" .
```

If found, update to new version. Common places:
- `main.go` (if you have a version variable)
- `README.md` (if version mentioned)
- Any documentation

If you made changes:
```bash
git add .
git commit -m "Bump version to v0.1.0"
git push origin main
```

### Step 3: Create Git Tag (2 minutes)

1. **Create annotated tag:**
   ```bash
   git tag -a v0.1.0 -m "Initial release of t6-asset-browser
   
   Features:
   - Parse BO2 FastFiles using OpenAssetTools
   - Extract weapons, perks, xmodels, materials, images
   - Export to GSC arrays for maxlib.gsc integration
   - Export to JSON, CSV, or plain text
   - Support for all 6 zombie maps (Origins, Mob, Buried, Die Rise, Tranzit, Nuketown)
   - Built-in caching for faster subsequent runs
   - Cross-platform: Linux, macOS, Windows
   
   Requirements:
   - OpenAssetTools must be installed"
   ```

2. **Verify tag created:**
   ```bash
   git tag | grep v0.1.0
   # Should output: v0.1.0
   ```

3. **Push tag to GitHub:**
   ```bash
   git push origin v0.1.0
   ```

4. **Verify on GitHub:**
   - Go to: https://github.com/maxvanasten/t6-asset-browser/tags
   - Should see `v0.1.0` in the list

### Step 4: Build Release Binaries (5 minutes)

1. **Run the release build script:**
   ```bash
   ./scripts/build-releases.sh
   ```

2. **Verify all 5 binaries were created:**
   ```bash
   ls -la dist/
   ```
   
   Expected output:
   ```
   t6-assets-darwin-arm64
   t6-assets-darwin-x64
   t6-assets-linux-arm64
   t6-assets-linux-x64
   t6-assets-win32-x64.exe
   ```

3. **Quick sanity check - test one binary:**
   ```bash
   ./dist/t6-assets-linux-x64 --help
   ```
   
   Should show help text without errors.

4. **Optional: Test with actual FastFiles**
   ```bash
   ./dist/t6-assets-linux-x64 -cmd=list -map=zm_tomb -type=weapon 2>&1 | head -5
   ```
   
   Should show weapon list.

### Step 5: Create GitHub Release (10 minutes)

1. **Navigate to GitHub:**
   - Go to: https://github.com/maxvanasten/t6-asset-browser
   - Click "Releases" in the right sidebar (or go directly to /releases)

2. **Create new release:**
   - Click green "Create a new release" button (or "Draft a new release")

3. **Select the tag:**
   - Click "Choose a tag" dropdown
   - Type `v0.1.0`
   - Select it from the dropdown

4. **Fill in release details:**
   
   **Release title:**
   ```
   v0.1.0 - Initial Release
   ```
   
   **Description** (copy and adapt this template):
   ```markdown
   ## What's New
   
   Initial release of T6 Asset Browser!
   
   ### Features
   - Parse BO2 FastFiles using OpenAssetTools integration
   - Extract complete weapon lists for all 6 zombie maps
   - Find all perks available on each map
   - Export assets to multiple formats:
     - GSC arrays (for maxlib.gsc integration)
     - JSON (for data processing)
     - CSV (for spreadsheets)
     - Plain text (for scripts)
   - Built-in caching for faster subsequent runs
   - Cross-platform support (Linux, macOS, Windows)
   
   ### Supported Maps
   - Origins (zm_tomb)
   - Mob of the Dead (zm_prison)
   - Buried (zm_buried)
   - Die Rise (zm_highrise)
   - Tranzit (zm_transit)
   - Nuketown (zm_nuked)
   
   ### Installation
   Download the appropriate binary for your platform from the Assets section below.
   
   See [README.md](https://github.com/maxvanasten/t6-asset-browser/blob/main/README.md) for detailed usage instructions.
   
   ### Requirements
   - OpenAssetTools must be installed: https://github.com/Laupetin/OpenAssetTools
   
   ### Example Usage
   ```bash
   # Export weapons from Origins as GSC array
   ./t6-assets -cmd=export -map=zm_tomb -type=weapon -format=gsc
   
    # Search for raygun variants across all maps
    ./t6-assets -cmd=search -pattern=raygun
   ```
   
   ### Assets
   - `t6-assets-linux-x64` - Linux (AMD64)
   - `t6-assets-linux-arm64` - Linux (ARM64)
   - `t6-assets-darwin-x64` - macOS (Intel)
   - `t6-assets-darwin-arm64` - macOS (Apple Silicon)
   - `t6-assets-win32-x64.exe` - Windows (AMD64)
   ```

5. **Upload binaries:**
   - In the "Attach binaries" section, click and drag all 5 files from `dist/` folder
   - Or click "selecting them" and choose the files
   - Wait for all uploads to complete (you'll see progress bars)
   - Verify all 5 files appear in the list

6. **Publish options:**
   - If this is a pre-release (beta/alpha), check "Set as a pre-release"
   - For stable releases, leave unchecked
   - Check "Set as the latest release" (should be default)

7. **Publish:**
   - Click green "Publish release" button

### Step 6: Verify Release (5 minutes)

1. **Check release page loads:**
   - Navigate to: https://github.com/maxvanasten/t6-asset-browser/releases/tag/v0.1.0
   - Page should load without errors

2. **Verify all assets attached:**
   - Scroll to "Assets" section
   - Should see all 5 binaries listed

3. **Test a download link:**
   ```bash
   curl -I https://github.com/maxvanasten/t6-asset-browser/releases/download/v0.1.0/t6-assets-linux-x64 2>&1 | head -5
   ```
   
   Should return HTTP 302 (redirect) or 200 OK

4. **Install test (Linux/macOS example):**
   ```bash
   cd /tmp
   curl -L -o t6-assets-test https://github.com/maxvanasten/t6-asset-browser/releases/download/v0.1.0/t6-assets-linux-x64
   chmod +x t6-assets-test
   ./t6-assets-test --help
   rm t6-assets-test
   ```

5. **Update README badge (if you added one):**
   If README has a version badge, it should auto-update on next page load.

### Step 7: Clean Up (2 minutes)

1. **Remove local dist folder (optional):**
   ```bash
   rm -rf dist/
   ```
   
   Note: This is gitignored, so won't affect repo.

2. **Return to development:**
   ```bash
   git checkout main
   ```

3. **Consider updating documentation:**
   - If this was a major release, update README screenshots/examples
   - Update CHANGELOG if you maintain one

---

## Emergency Procedures

### Fixing a Bad Release

If you made a mistake in the release:

1. **Delete the GitHub release:**
   - Go to release page
   - Click "Delete" button (red, near bottom)
   - Confirm deletion

2. **Delete the tag locally and remotely:**
   ```bash
   # Delete local tag
   git tag -d v0.1.0
   
   # Delete remote tag
   git push origin :refs/tags/v0.1.0
   ```

3. **Fix the issue:**
   - Make code changes
   - Commit and push
   - Run tests

4. **Start over from Step 3** (Create Git Tag)

### Binary is Corrupted

If a binary doesn't work:

1. Rebuild that specific platform:
   ```bash
   GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o dist/t6-assets-linux-x64 ./cmd/t6-assets
   ```

2. Test the binary:
   ```bash
   ./dist/t6-assets-linux-x64 --help
   ```

3. Edit the release on GitHub:
   - Go to release page
   - Click "Edit" button
   - Delete the bad asset
   - Upload the fixed binary
   - Click "Update release"

---

## Version Number Guide

### When to Increment

**Patch (0.0.1 -> 0.0.2):**
- Bug fixes
- Performance improvements
- Documentation updates

**Minor (0.1.0 -> 0.2.0):**
- New features added
- New asset types supported
- New export formats

**Major (0.x.x -> 1.0.0):**
- Breaking changes to CLI interface
- Complete rewrite
- Stable, production-ready milestone

### Examples

```
v0.1.0 - Initial release
v0.1.1 - Fixed bug in weapon filtering
v0.1.2 - Performance improvements
v0.2.0 - Added perk detection, JSON export
v0.2.1 - Fixed OAT integration bug
v0.3.0 - Added material/image support
...
v1.0.0 - Stable release
```

---

## Pre-Release Checklist

Before starting any release, verify:

- [ ] On main branch
- [ ] Git status clean
- [ ] All tests pass (`go test ./...`)
- [ ] Code compiles without warnings
- [ ] README is up to date
- [ ] Version number decided
- [ ] OpenAssetTools available for testing
- [ ] GitHub access confirmed

---

## Post-Release Checklist

After publishing:

- [ ] Release page loads correctly
- [ ] All 5 binaries downloadable
- [ ] At least one binary tested
- [ ] Tag visible on GitHub
- [ ] Local cleanup done
- [ ] Ready for next development cycle

---

## Questions?

If you're unsure about any step:
1. Check this document again
2. Look at previous releases as examples: https://github.com/maxvanasten/t6-asset-browser/releases
3. Refer to the gsclsp release process (similar workflow)

---

**Remember:** It's better to take your time and get it right than to rush and fix mistakes later.

**Happy releasing!** 🚀
