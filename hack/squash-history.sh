#!/usr/bin/env fish

set branch (git branch --show-current)
set root (git rev-list --max-parents=0 HEAD)
echo "Root commit: $root"
echo "Squashing all commits into one..."

git reset --soft $root
git commit --amend -m "initial commit"
git push origin $branch --force

echo "Done. History squashed into one commit."
