#!/usr/bin/env fish

# Delete all GitHub releases
echo "Deleting GitHub releases..."
for release in (gh release list --limit 100 --json tagName --jq '.[].tagName')
    echo "  Deleting release: $release"
    gh release delete $release --yes
end

# Delete all remote tags
echo "Deleting remote tags..."
for tag in (git tag)
    echo "  Deleting remote tag: $tag"
    git push origin --delete $tag
end

# Delete all local tags
echo "Deleting local tags..."
git tag | xargs -r git tag -d

# Delete all packages belonging to this repo
echo "Deleting GitHub packages..."
set repo (gh repo view --json nameWithOwner --jq '.nameWithOwner')
set owner (echo $repo | cut -d'/' -f1)
set repo_name (echo $repo | cut -d'/' -f2)

for pkg in (gh api "/users/$owner/packages?package_type=container" --jq ".[] | select(.repository.name == \"$repo_name\") | .name" 2>/dev/null)
    echo "  Deleting package: $pkg"
    gh api --method DELETE "/users/$owner/packages/container/$pkg"
end

echo "Done."
