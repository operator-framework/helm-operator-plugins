# Release process for `helm-operator-plugins`

1. Creation of pre-release PR:

Update the release version of the plugin at the following locations in the file - `pkg/plugins/hybrid/v1alpha/scaffolds/init.go`. As an example, let us assume that we are about to update the release version from `v0.0.9` to `v0.0.10`. To do so, modify the `hybridOperatorVersion` and `helmPluginVersion` variables to `v0.0.10`.

```diff
-	hybridOperatorVersion = "0.0.9"
+ 	hybridOperatorVersion = "0.0.10"

 	// helmPluginVersion is the operator-framework/helm-operator-plugin version to be used in the project
- 	helmPluginVersion = "v0.0.9"
+ 	helmPluginVersion = "v0.0.10"
```

Run `make generate` to re-generate the `testdata/` with above modifications.

**Note**
The release PR with the above mentioned change will **fail** the `sanity` test and needs to be merged forcefully. This step is done because the packages which are to be imported in the scaffolded project and the plugin code reside in the same repository. This will ensure that after the release, the plugin will also import the updated version of library helpers.

2. Create a release branch with the name vX.Y.x. Example: `git checkout -b v0.0.10`.

3. Tag the release commit and verify the generated tag.

```
export VER=v0.0.10
git tag --sign --message "helm-operator-plugin $VER" "$VER"
git verify-tag --verbose $VER
```

4. Push the release branch and tag:

```
git push upstream <release-branch>
git push upstream $VER
```

5. Make sure that the release notes are updated in the github release page (Github provides an option to auto-generate the notes).

6. After the release, make sure to run `make test-sanity` from the main branch to ensure that `testdata` is up-to date. If not, create a PR to update the changes.